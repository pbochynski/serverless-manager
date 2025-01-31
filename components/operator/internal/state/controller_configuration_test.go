package state

import (
	"context"
	"testing"

	"github.com/kyma-project/serverless/components/operator/api/v1alpha1"
	"github.com/kyma-project/serverless/components/operator/internal/chart"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_sFnControllerConfiguration(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	configurationReadyMsg := "Configuration ready"

	t.Run("update status additional configuration overrides", func(t *testing.T) {
		s := &systemState{
			instance: v1alpha1.Serverless{
				Spec: v1alpha1.ServerlessSpec{
					TargetCPUUtilizationPercentage:   cpuUtilizationTest,
					FunctionRequeueDuration:          requeueDurationTest,
					FunctionBuildExecutorArgs:        executorArgsTest,
					FunctionBuildMaxSimultaneousJobs: maxSimultaneousJobsTest,
					HealthzLivenessTimeout:           healthzLivenessTimeoutTest,
					FunctionRequestBodyLimitMb:       requestBodyLimitMbTest,
					FunctionTimeoutSec:               timeoutSecTest,
					DefaultBuildJobPreset:            defaultBuildJobPresetTest,
					DefaultRuntimePodPreset:          defaultRuntimePodPresetTest,
				},
			},
			flagsBuilder: chart.NewFlagsBuilder(),
		}

		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		eventRecorder := record.NewFakeRecorder(10)
		r := &reconciler{log: zap.NewNop().Sugar(), k8s: k8s{client: c, EventRecorder: eventRecorder}}
		next, result, err := sFnControllerConfiguration(context.TODO(), r, s)
		require.Nil(t, err)
		require.Nil(t, result)
		requireEqualFunc(t, sFnApplyResources, next)

		status := s.instance.Status
		require.Equal(t, cpuUtilizationTest, status.CPUUtilizationPercentage)
		require.Equal(t, requeueDurationTest, status.RequeueDuration)
		require.Equal(t, executorArgsTest, status.BuildExecutorArgs)
		require.Equal(t, maxSimultaneousJobsTest, status.BuildMaxSimultaneousJobs)
		require.Equal(t, healthzLivenessTimeoutTest, status.HealthzLivenessTimeout)
		require.Equal(t, requestBodyLimitMbTest, status.RequestBodyLimitMb)
		require.Equal(t, timeoutSecTest, status.TimeoutSec)
		require.Equal(t, defaultBuildJobPresetTest, status.DefaultBuildJobPreset)
		require.Equal(t, defaultRuntimePodPresetTest, status.DefaultRuntimePodPreset)

		require.Equal(t, v1alpha1.StateProcessing, status.State)
		requireContainsCondition(t, status,
			v1alpha1.ConditionTypeConfigured,
			metav1.ConditionTrue,
			v1alpha1.ConditionReasonConfigured,
			configurationReadyMsg,
		)

		expectedEvents := []string{
			"Normal Configuration CPU utilization set from '' to 'test-CPU-utilization-percentage'",
			"Normal Configuration Function requeue duration set from '' to 'test-requeue-duration'",
			"Normal Configuration Function build executor args set from '' to 'test-build-executor-args'",
			"Normal Configuration Max number of simultaneous jobs set from '' to 'test-max-simultaneous-jobs'",
			"Normal Configuration Duration of health check set from '' to 'test-healthz-liveness-timeout'",
			"Normal Configuration Max size of request body set from '' to 'test-request-body-limit-mb'",
			"Normal Configuration Timeout set from '' to 'test-timeout-sec'",
			"Normal Configuration Default build job preset set from '' to 'test=default-build-job-preset'",
			"Normal Configuration Default runtime pod preset set from '' to 'test-default-runtime-pod-preset'",
		}

		for _, expectedEvent := range expectedEvents {
			require.Equal(t, expectedEvent, <-eventRecorder.Events)
		}
	})

	t.Run("reconcile from configurationError", func(t *testing.T) {
		s := &systemState{
			instance: v1alpha1.Serverless{
				Status: v1alpha1.ServerlessStatus{
					Conditions: []metav1.Condition{
						{
							Type:   string(v1alpha1.ConditionTypeConfigured),
							Status: metav1.ConditionFalse,
							Reason: string(v1alpha1.ConditionReasonConfigurationErr),
						},
						{
							Type:   string(v1alpha1.ConditionTypeInstalled),
							Status: metav1.ConditionTrue,
							Reason: string(v1alpha1.ConditionReasonInstallation),
						},
					},
					State:            v1alpha1.StateError,
					EventingEndpoint: "test-event-URL",
					TracingEndpoint:  v1alpha1.EndpointDisabled,
				},
				Spec: v1alpha1.ServerlessSpec{
					Eventing: &v1alpha1.Endpoint{Endpoint: "test-event-URL"},
					Tracing:  &v1alpha1.Endpoint{Endpoint: v1alpha1.EndpointDisabled},
					DockerRegistry: &v1alpha1.DockerRegistry{
						EnableInternal: pointer.Bool(false),
						SecretName:     pointer.String("boo"),
					},
				},
			},
			statusSnapshot: v1alpha1.ServerlessStatus{
				DockerRegistry: "",
			},
			flagsBuilder: chart.NewFlagsBuilder(),
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "boo",
			},
		}
		r := &reconciler{
			log: zap.NewNop().Sugar(),
			k8s: k8s{
				client: fake.NewClientBuilder().WithObjects(secret).Build(),
			},
		}

		next, result, err := sFnControllerConfiguration(context.Background(), r, s)
		require.NoError(t, err)
		require.Nil(t, result)
		requireEqualFunc(t, sFnApplyResources, next)
		requireContainsCondition(t, s.instance.Status,
			v1alpha1.ConditionTypeConfigured,
			metav1.ConditionTrue,
			v1alpha1.ConditionReasonConfigured,
			configurationReadyMsg)
		require.Equal(t, v1alpha1.StateProcessing, s.instance.Status.State)
	})
}
