// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"context"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/diki/pkg/kubernetes/config"
	kubeutils "github.com/gardener/diki/pkg/kubernetes/utils"
	"github.com/gardener/diki/pkg/rule"
)

var (
	_ rule.Rule     = &Rule242379{}
	_ rule.Severity = &Rule242379{}
)

type Rule242379 struct {
	Client                client.Client
	Namespace             string
	StatefulSetETCDMain   string
	StatefulSetETCDEvents string
}

func (r *Rule242379) ID() string {
	return ID242379
}

func (r *Rule242379) Name() string {
	return "The Kubernetes etcd must use TLS to protect the confidentiality of sensitive data during electronic dissemination."
}

func (r *Rule242379) Severity() rule.SeverityLevel {
	return rule.SeverityMedium
}

func (r *Rule242379) Run(ctx context.Context) (rule.RuleResult, error) {
	var checkResults []rule.CheckResult
	etcdMain := "etcd-main"
	etcdEvents := "etcd-events"

	if r.StatefulSetETCDMain != "" {
		etcdMain = r.StatefulSetETCDMain
	}

	if r.StatefulSetETCDEvents != "" {
		etcdEvents = r.StatefulSetETCDEvents
	}

	checkResults = append(checkResults, r.checkStatefulSet(ctx, etcdMain))
	checkResults = append(checkResults, r.checkStatefulSet(ctx, etcdEvents))

	return rule.Result(r, checkResults...), nil
}

func (r *Rule242379) checkStatefulSet(ctx context.Context, statefulSetName string) rule.CheckResult {
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: r.Namespace,
		},
	}

	target := kubeutils.TargetWithK8sObject(rule.NewTarget(), metav1.TypeMeta{Kind: "StatefulSet"}, statefulSet.ObjectMeta)

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet); err != nil {
		return rule.ErroredCheckResult(err.Error(), target)
	}

	volume, found := kubeutils.GetVolumeFromStatefulSet(statefulSet, "etcd-config-file")
	if !found {
		return rule.ErroredCheckResult("StatefulSet does not contain volume with name: etcd-config-file.", target)
	}

	configByteSlice, err := kubeutils.GetFileDataFromVolume(ctx, r.Client, r.Namespace, volume, "etcd.conf.yaml")
	if err != nil {
		return rule.ErroredCheckResult(err.Error(), target)
	}

	config := &config.EtcdConfig{}
	err = yaml.Unmarshal(configByteSlice, config)
	if err != nil {
		return rule.ErroredCheckResult(err.Error(), target)
	}

	if config.ClientTransportSecurity.AutoTLS == nil {
		return rule.WarningCheckResult("Option client-transport-security.auto-tls has not been set.", target)
	}
	if *config.ClientTransportSecurity.AutoTLS {
		return rule.FailedCheckResult("Option client-transport-security.auto-tls set to not allowed value.", target)
	}

	return rule.PassedCheckResult("Option client-transport-security.auto-tls set to allowed value.", target)
}
