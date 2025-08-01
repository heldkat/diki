// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/diki/pkg/provider/managedk8s/ruleset/disak8sstig/rules"
	"github.com/gardener/diki/pkg/rule"
	"github.com/gardener/diki/pkg/shared/ruleset/disak8sstig/option"
)

var _ = Describe("#242415", func() {
	var (
		fakeClient    client.Client
		options       *option.Options242415
		pod           *corev1.Pod
		ctx           = context.TODO()
		namespaceName = "foo"
		namespace     *corev1.Namespace
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().Build()

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
				Labels: map[string]string{
					"foo": "bar",
				},
			},
		}

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: namespaceName,
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "test",
						Env:  []corev1.EnvVar{},
					},
				},
			},
		}
		options = &option.Options242415{}
	})

	It("should pass when no pods are deployed", func() {
		r := &rules.Rule242415{Client: fakeClient, Options: options}

		ruleResult, err := r.Run(ctx)
		Expect(err).ToNot(HaveOccurred())

		expectedCheckResults := []rule.CheckResult{
			{
				Status:  rule.Passed,
				Message: "The cluster does not have any Pods.",
				Target:  rule.NewTarget(),
			},
		}

		Expect(ruleResult.CheckResults).To(Equal(expectedCheckResults))
	})

	It("should return correct results when all pods pass", func() {
		r := &rules.Rule242415{Client: fakeClient, Options: options}
		Expect(fakeClient.Create(ctx, pod)).To(Succeed())

		ruleResult, err := r.Run(ctx)
		Expect(err).ToNot(HaveOccurred())

		expectedCheckResults := []rule.CheckResult{
			{
				Status:  rule.Passed,
				Message: "Pod does not use environment to inject secret.",
				Target:  rule.NewTarget("name", "pod", "namespace", "foo", "kind", "Pod"),
			},
		}

		Expect(ruleResult.CheckResults).To(Equal(expectedCheckResults))
	})

	It("should return correct results when a pod fails", func() {
		r := &rules.Rule242415{Client: fakeClient, Options: options}
		pod.Spec.Containers[0].Env = []corev1.EnvVar{
			{
				Name: "SECRET_TEST",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "secret_test",
					},
				},
			},
		}
		Expect(fakeClient.Create(ctx, pod)).To(Succeed())

		ruleResult, err := r.Run(ctx)
		Expect(err).ToNot(HaveOccurred())

		expectedCheckResults := []rule.CheckResult{
			{
				Status:  rule.Failed,
				Message: "Pod uses environment to inject secret.",
				Target:  rule.NewTarget("name", "pod", "namespace", "foo", "kind", "Pod", "container", "test", "details", "variableName: SECRET_TEST, keyRef: secret_test"),
			},
		}

		Expect(ruleResult.CheckResults).To(Equal(expectedCheckResults))
	})

	It("should return correct results when a pod contains an initContainer", func() {
		r := &rules.Rule242415{Client: fakeClient, Options: options}
		pod.Spec.InitContainers = []corev1.Container{
			{
				Name: "initFoo",
				Env: []corev1.EnvVar{
					{
						Name: "SECRET_TEST",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key: "secret_test",
							},
						},
					},
				},
			},
		}
		Expect(fakeClient.Create(ctx, pod)).To(Succeed())

		ruleResult, err := r.Run(ctx)
		Expect(err).ToNot(HaveOccurred())

		expectedCheckResults := []rule.CheckResult{
			{
				Status:  rule.Failed,
				Message: "Pod uses environment to inject secret.",
				Target:  rule.NewTarget("name", "pod", "namespace", "foo", "kind", "Pod", "container", "initFoo", "details", "variableName: SECRET_TEST, keyRef: secret_test"),
			},
		}

		Expect(ruleResult.CheckResults).To(Equal(expectedCheckResults))
	})

	It("should return correct results when a pod has accepted environment variables", func() {
		options = &option.Options242415{
			AcceptedPods: []option.AcceptedPods242415{
				{
					PodSelector: option.PodSelector{
						PodMatchLabels:       map[string]string{"foo": "bar"},
						NamespaceMatchLabels: map[string]string{"foo": "bar"},
					},
					EnvironmentVariables: []string{"SECRET_TEST"},
				},
			},
		}
		r := &rules.Rule242415{Client: fakeClient, Options: options}
		pod.Spec.Containers[0].Env = []corev1.EnvVar{
			{
				Name: "SECRET_TEST",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "secret_test",
					},
				},
			},
		}

		Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
		Expect(fakeClient.Create(ctx, pod)).To(Succeed())

		ruleResult, err := r.Run(ctx)
		Expect(err).ToNot(HaveOccurred())

		expectedCheckResults := []rule.CheckResult{
			{
				Status:  rule.Accepted,
				Message: "Pod accepted to use environment to inject secret.",
				Target:  rule.NewTarget("name", "pod", "namespace", "foo", "kind", "Pod", "container", "test", "details", "variableName: SECRET_TEST, keyRef: secret_test"),
			},
		}

		Expect(ruleResult.CheckResults).To(Equal(expectedCheckResults))
	})

	It("should return correct targets when the pods have owner references", func() {
		options = &option.Options242415{}

		r := &rules.Rule242415{Client: fakeClient, Options: options}

		Expect(fakeClient.Create(ctx, namespace)).To(Succeed())

		replicaSet := appsv1.ReplicaSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ReplicaSet",
				APIVersion: "apps/v1",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name:      "replicaSet",
				UID:       "1",
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "deployment",
					},
				},
			},
		}
		Expect(fakeClient.Create(ctx, &replicaSet)).To(Succeed())

		pod1 := pod.DeepCopy()
		pod1.Name = "pod1"
		pod1.OwnerReferences = []metav1.OwnerReference{
			{
				UID:        "3",
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "deployment",
			},
		}
		Expect(fakeClient.Create(ctx, pod1)).To(Succeed())

		pod2 := pod.DeepCopy()
		pod2.Name = "pod2"
		pod2.OwnerReferences = []metav1.OwnerReference{
			{
				UID:        "3",
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "deployment",
			},
		}
		Expect(fakeClient.Create(ctx, pod2)).To(Succeed())

		pod3 := pod.DeepCopy()
		pod3.Name = "pod3"
		pod3.OwnerReferences = []metav1.OwnerReference{
			{
				UID:        "4",
				Kind:       "DaemonSet",
				APIVersion: "apps/v1",
				Name:       "daemonSet",
			},
		}
		Expect(fakeClient.Create(ctx, pod3)).To(Succeed())

		ruleResult, err := r.Run(ctx)
		Expect(err).ToNot(HaveOccurred())

		expectedCheckResults := []rule.CheckResult{
			{
				Status:  rule.Passed,
				Message: "Pod does not use environment to inject secret.",
				Target:  rule.NewTarget("name", "deployment", "namespace", "foo", "kind", "Deployment"),
			},
			{
				Status:  rule.Passed,
				Message: "Pod does not use environment to inject secret.",
				Target:  rule.NewTarget("name", "daemonSet", "namespace", "foo", "kind", "DaemonSet"),
			},
		}

		Expect(ruleResult.CheckResults).To(Equal(expectedCheckResults))
	})

})
