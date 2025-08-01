// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/diki/pkg/rule"
	"github.com/gardener/diki/pkg/shared/ruleset/disak8sstig/rules"
)

var _ = Describe("#242428", func() {
	const (
		ctsCertAuthNotSetConfig = `
client-transport-security:`
		ctsCertAuthSetFalseConfig = `
client-transport-security:
  client-cert-auth: false`
		ctsCertAuthSetTrueConfig = `
client-transport-security:
  client-cert-auth: true`
		ctsCertFileNotSetConfig = `
client-transport-security:`
		ctsCertFileSetConfig = `
client-transport-security:
  cert-file: set`
		ctsCertFileSetEmptyConfig = `
client-transport-security:
  cert-file: ""`
	)
	var (
		fakeClient client.Client
		ctx        = context.TODO()
		namespace  = "foo"

		etcdMainStatefulSet   *appsv1.StatefulSet
		etcdEventsStatefulSet *appsv1.StatefulSet
		targetEtcdMain        = rule.NewTarget("name", "etcd-main", "namespace", namespace, "kind", "StatefulSet")
		targetEtcdEvents      = rule.NewTarget("name", "etcd-events", "namespace", namespace, "kind", "StatefulSet")
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().Build()
		etcdMainStatefulSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-main",
				Namespace: namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{},
					},
				},
			},
		}
		etcdEventsStatefulSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-events",
				Namespace: namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{},
					},
				},
			},
		}
	})

	It("should return error check results when etcd-main and etcd-events are not found", func() {
		r := &rules.Rule242428{Client: fakeClient, Namespace: namespace}

		ruleResult, err := r.Run(ctx)
		Expect(err).ToNot(HaveOccurred())

		Expect(ruleResult.CheckResults).To(Equal([]rule.CheckResult{
			{
				Status:  rule.Errored,
				Message: "statefulsets.apps \"etcd-main\" not found",
				Target:  targetEtcdMain,
			},
			{
				Status:  rule.Errored,
				Message: "statefulsets.apps \"etcd-events\" not found",
				Target:  targetEtcdEvents,
			},
		},
		))
	})

	DescribeTable("Run cases",
		func(etcdMainVolume, etcdEventsVolume corev1.Volume, etcdMainSecret, etcdEventsSecret *corev1.Secret, expectedCheckResults []rule.CheckResult, errorMatcher gomegatypes.GomegaMatcher) {
			etcdMainStatefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{etcdMainVolume}
			Expect(fakeClient.Create(ctx, etcdMainStatefulSet)).To(Succeed())

			etcdEventsStatefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{etcdEventsVolume}
			Expect(fakeClient.Create(ctx, etcdEventsStatefulSet)).To(Succeed())

			Expect(fakeClient.Create(ctx, etcdMainSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, etcdEventsSecret)).To(Succeed())

			r := &rules.Rule242428{Client: fakeClient, Namespace: namespace}
			ruleResult, err := r.Run(ctx)
			Expect(err).To(errorMatcher)

			Expect(ruleResult.CheckResults).To(Equal(expectedCheckResults))
		},

		Entry("should fail when client-transport-security.cert-file is not set",
			corev1.Volume{Name: "etcd-config-file", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "foo"}}},
			corev1.Volume{Name: "etcd-config-file", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "bar"}}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace}, Data: map[string][]byte{"etcd.conf.yaml": []byte(ctsCertFileNotSetConfig)}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: namespace}, Data: map[string][]byte{"etcd.conf.yaml": []byte(ctsCertFileSetEmptyConfig)}},
			[]rule.CheckResult{
				{
					Status:  rule.Failed,
					Message: "Option client-transport-security.cert-file has not been set.",
					Target:  targetEtcdMain,
				},
				{
					Status:  rule.Failed,
					Message: "Option client-transport-security.cert-file is empty.",
					Target:  targetEtcdEvents,
				},
			},
			BeNil()),
		Entry("should fail when client-transport-security.cert-file is set",
			corev1.Volume{Name: "etcd-config-file", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "foo"}}},
			corev1.Volume{Name: "etcd-config-file", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "bar"}}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace}, Data: map[string][]byte{"etcd.conf.yaml": []byte(ctsCertFileSetConfig)}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: namespace}, Data: map[string][]byte{"etcd.conf.yaml": []byte(ctsCertFileSetConfig)}},
			[]rule.CheckResult{
				{
					Status:  rule.Passed,
					Message: "Option client-transport-security.cert-file set to allowed value.",
					Target:  targetEtcdMain,
				},
				{
					Status:  rule.Passed,
					Message: "Option client-transport-security.cert-file set to allowed value.",
					Target:  targetEtcdEvents,
				},
			},
			BeNil()),
		Entry("should error when statefulSet does not have volume 'etcd-config-file' or secret is not found",
			corev1.Volume{Name: "not-etcd-config-file", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "foo"}}},
			corev1.Volume{Name: "etcd-config-file", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "bar"}}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace}, Data: map[string][]byte{"etcd.conf.yaml": []byte(ctsCertAuthSetTrueConfig)}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "not-bar", Namespace: namespace}, Data: map[string][]byte{"etcd.conf.yaml": []byte(ctsCertAuthSetTrueConfig)}},
			[]rule.CheckResult{
				{
					Status:  rule.Errored,
					Message: "StatefulSet does not contain volume with name: etcd-config-file.",
					Target:  targetEtcdMain,
				},
				{
					Status:  rule.Errored,
					Message: "secrets \"bar\" not found",
					Target:  targetEtcdEvents,
				},
			},
			BeNil()),
	)
})
