/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

//+kubebuilder:webhook:path=/mutate-core-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups=core,resources=pods,verbs=create,versions=v1,name=mpod.kb.io,admissionReviewVersions=v1

// PodAntiAffiniytMutate mutate Pods
type PodAntiAffiniytMutate struct {
	Client  client.Client
	decoder *admission.Decoder
	logger  logr.Logger
}

func NewPodAffiniytMutate(c client.Client, d *admission.Decoder, log logr.Logger) admission.Handler {
	return &PodAntiAffiniytMutate{
		Client:  c,
		decoder: d,
		logger:  log}
}

const (
	podAnnotationsRedisClusterApp = "redis.opstreelabs.instance"
	podLabelsPodName              = "statefulset.kubernetes.io/pod-name"
)

func (v *PodAntiAffiniytMutate) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := v.logger.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	pod := &corev1.Pod{}
	err := v.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if v.isRedisClusterApp(pod) == "" {
		return admission.Allowed("")
	}

	old := pod.DeepCopy()

	v.AddPodAntiAffinity(pod)
	if !reflect.DeepEqual(old, pod) {
		marshaledPod, err := json.Marshal(pod)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		logger.Info("mutate pod with anti-affinity")
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
	}

	return admission.Allowed("")
}

// PodAntiAffiniytMutate implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (v *PodAntiAffiniytMutate) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

func (m *PodAntiAffiniytMutate) InjectLogger(l logr.Logger) error {
	m.logger = l
	return nil
}

func (v *PodAntiAffiniytMutate) AddPodAntiAffinity(pod *corev1.Pod) {
	// todo: determine whether to add anti affinity,need add parameters to control

	podName := pod.ObjectMeta.Name
	antiLabelValue := v.getAntiAffinityValue(podName)

	if pod.Spec.Affinity == nil {
		pod.Spec.Affinity = &corev1.Affinity{}
	}
	if pod.Spec.Affinity.PodAntiAffinity == nil {
		pod.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}
	if pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = make([]corev1.PodAffinityTerm, 0)
	}
	addAntiAffinity := corev1.PodAffinityTerm{
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      podLabelsPodName,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{antiLabelValue},
				},
			},
		},
		TopologyKey: "kubernetes.io/hostname",
	}

	pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, addAntiAffinity)
}

func (v *PodAntiAffiniytMutate) getPodAnnotations(pod *corev1.Pod) map[string]string {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	return pod.Annotations
}

func (v *PodAntiAffiniytMutate) isRedisClusterApp(pod *corev1.Pod) string {
	annotations := v.getPodAnnotations(pod)
	return annotations[podAnnotationsRedisClusterApp]
}

func (v *PodAntiAffiniytMutate) getAntiAffinityValue(podName string) string {
	if strings.Contains(podName, "follower") {
		return strings.Replace(podName, "follower", "leader", -1)
	}
	if strings.Contains(podName, "leader") {
		return strings.Replace(podName, "leader", "follower", -1)
	}
	return ""
}
