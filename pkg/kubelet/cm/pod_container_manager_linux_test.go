//go:build linux

/*
Copyright 2016 The Kubernetes Authors.

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

package cm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestIsCgroupPod(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	qosContainersInfo := QOSContainersInfo{
		Guaranteed: RootCgroupName,
		Burstable:  NewCgroupName(RootCgroupName, strings.ToLower(string(v1.PodQOSBurstable))),
		BestEffort: NewCgroupName(RootCgroupName, strings.ToLower(string(v1.PodQOSBestEffort))),
	}
	podUID := types.UID("123")
	testCases := []struct {
		input          CgroupName
		expectedResult bool
		expectedUID    types.UID
	}{
		{
			input:          RootCgroupName,
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(qosContainersInfo.Guaranteed),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(qosContainersInfo.Guaranteed, GetPodCgroupNameSuffix(podUID)),
			expectedResult: true,
			expectedUID:    podUID,
		},
		{
			input:          NewCgroupName(qosContainersInfo.Guaranteed, GetPodCgroupNameSuffix(podUID), "container.scope"),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(qosContainersInfo.Burstable),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(qosContainersInfo.Burstable, GetPodCgroupNameSuffix(podUID)),
			expectedResult: true,
			expectedUID:    podUID,
		},
		{
			input:          NewCgroupName(qosContainersInfo.Burstable, GetPodCgroupNameSuffix(podUID), "container.scope"),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(qosContainersInfo.BestEffort),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(qosContainersInfo.BestEffort, GetPodCgroupNameSuffix(podUID)),
			expectedResult: true,
			expectedUID:    podUID,
		},
		{
			input:          NewCgroupName(qosContainersInfo.BestEffort, GetPodCgroupNameSuffix(podUID), "container.scope"),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(RootCgroupName, "system"),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			input:          NewCgroupName(RootCgroupName, "system", "kubelet"),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
		{
			// contains reserved word "pod" in cgroup name
			input:          NewCgroupName(RootCgroupName, GetPodCgroupNameSuffix("this-uid-contains-reserved-word-pod")),
			expectedResult: false,
			expectedUID:    types.UID(""),
		},
	}
	for _, cgroupDriver := range []string{"cgroupfs", "systemd"} {
		pcm := &podContainerManagerImpl{
			cgroupManager:     NewCgroupManager(logger, nil, cgroupDriver),
			enforceCPULimits:  true,
			qosContainersInfo: qosContainersInfo,
		}
		for _, testCase := range testCases {
			// Give the right cgroup structure based on whether systemd is enabled.
			var name string
			if cgroupDriver == "systemd" {
				name = testCase.input.ToSystemd()
			} else {
				name = testCase.input.ToCgroupfs()
			}
			// check if this is a pod or not with the literal cgroupfs input
			result, resultUID := pcm.IsPodCgroup(name)
			if result != testCase.expectedResult {
				t.Errorf("Unexpected result for driver: %v, input: %v, expected: %v, actual: %v", cgroupDriver, testCase.input, testCase.expectedResult, result)
			}
			if resultUID != testCase.expectedUID {
				t.Errorf("Unexpected result for driver: %v, input: %v, expected: %v, actual: %v", cgroupDriver, testCase.input, testCase.expectedUID, resultUID)
			}

		}
	}
}

func TestGetPodContainerName(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	newGuaranteedPodWithUID := func(uid types.UID) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID: uid,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "container",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1000m"),
								v1.ResourceMemory: resource.MustParse("1G"),
							},
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1000m"),
								v1.ResourceMemory: resource.MustParse("1G"),
							},
						},
					},
				},
			},
		}
	}
	newBurstablePodWithUID := func(uid types.UID) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID: uid,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "container",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1000m"),
								v1.ResourceMemory: resource.MustParse("1G"),
							},
						},
					},
				},
			},
		}
	}
	newBestEffortPodWithUID := func(uid types.UID) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID: uid,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "container",
					},
				},
			},
		}
	}

	qosContainersInfo := QOSContainersInfo{
		Guaranteed: RootCgroupName,
		Burstable:  NewCgroupName(RootCgroupName, strings.ToLower(string(v1.PodQOSBurstable))),
		BestEffort: NewCgroupName(RootCgroupName, strings.ToLower(string(v1.PodQOSBestEffort))),
	}

	type fields struct {
		cgroupManager CgroupManager
	}
	type args struct {
		pod *v1.Pod
	}

	tests := []struct {
		name                string
		fields              fields
		args                args
		wantCgroupName      CgroupName
		wantLiteralCgroupfs string
	}{
		{
			name: "pod with qos guaranteed and cgroupfs",
			fields: fields{
				cgroupManager: NewCgroupManager(logger, nil, "cgroupfs"),
			},
			args: args{
				pod: newGuaranteedPodWithUID("fake-uid-1"),
			},
			wantCgroupName:      NewCgroupName(qosContainersInfo.Guaranteed, "podfake-uid-1"),
			wantLiteralCgroupfs: NewCgroupName(qosContainersInfo.Guaranteed, "podfake-uid-1").ToCgroupfs(),
		}, {
			name: "pod with qos guaranteed and systemd",
			fields: fields{
				cgroupManager: NewCgroupManager(logger, nil, "systemd"),
			},
			args: args{
				pod: newGuaranteedPodWithUID("fake-uid-2"),
			},
			wantCgroupName:      NewCgroupName(qosContainersInfo.Guaranteed, "podfake-uid-2"),
			wantLiteralCgroupfs: NewCgroupName(qosContainersInfo.Guaranteed, "podfake-uid-2").ToSystemd(),
		}, {
			name: "pod with qos burstable and cgroupfs",
			fields: fields{
				cgroupManager: NewCgroupManager(logger, nil, "cgroupfs"),
			},
			args: args{
				pod: newBurstablePodWithUID("fake-uid-3"),
			},
			wantCgroupName:      NewCgroupName(qosContainersInfo.Burstable, "podfake-uid-3"),
			wantLiteralCgroupfs: NewCgroupName(qosContainersInfo.Burstable, "podfake-uid-3").ToCgroupfs(),
		}, {
			name: "pod with qos burstable and systemd",
			fields: fields{
				cgroupManager: NewCgroupManager(logger, nil, "systemd"),
			},
			args: args{
				pod: newBurstablePodWithUID("fake-uid-4"),
			},
			wantCgroupName:      NewCgroupName(qosContainersInfo.Burstable, "podfake-uid-4"),
			wantLiteralCgroupfs: NewCgroupName(qosContainersInfo.Burstable, "podfake-uid-4").ToSystemd(),
		}, {
			name: "pod with qos best-effort and cgroupfs",
			fields: fields{
				cgroupManager: NewCgroupManager(logger, nil, "cgroupfs"),
			},
			args: args{
				pod: newBestEffortPodWithUID("fake-uid-5"),
			},
			wantCgroupName:      NewCgroupName(qosContainersInfo.BestEffort, "podfake-uid-5"),
			wantLiteralCgroupfs: NewCgroupName(qosContainersInfo.BestEffort, "podfake-uid-5").ToCgroupfs(),
		}, {
			name: "pod with qos best-effort and systemd",
			fields: fields{
				cgroupManager: NewCgroupManager(logger, nil, "systemd"),
			},
			args: args{
				pod: newBestEffortPodWithUID("fake-uid-6"),
			},
			wantCgroupName:      NewCgroupName(qosContainersInfo.BestEffort, "podfake-uid-6"),
			wantLiteralCgroupfs: NewCgroupName(qosContainersInfo.BestEffort, "podfake-uid-6").ToSystemd(),
		},
	}

	for _, tt := range tests {
		pcm := &podContainerManagerImpl{
			cgroupManager:     tt.fields.cgroupManager,
			qosContainersInfo: qosContainersInfo,
		}

		t.Run(tt.name, func(t *testing.T) {
			actualCgroupName, actualLiteralCgroupfs := pcm.GetPodContainerName(tt.args.pod)
			require.Equalf(t, tt.wantCgroupName, actualCgroupName, "Unexpected cgroup name for pod with UID %s, container resources: %v", tt.args.pod.UID, tt.args.pod.Spec.Containers[0].Resources)
			require.Equalf(t, tt.wantLiteralCgroupfs, actualLiteralCgroupfs, "Unexpected literal cgroupfs for pod with UID %s, container resources: %v", tt.args.pod.UID, tt.args.pod.Spec.Containers[0].Resources)
		})
	}
}

type fakePCMTestCgroupManager struct {
	exists       map[string]bool
	createErr    error
	updateErr    error
	destroyErr   error
	createCalls  int
	updateCalls  int
	destroyCalls int
}

func (f *fakePCMTestCgroupManager) Create(_ klog.Logger, config *CgroupConfig) error {
	f.createCalls++
	if f.createErr != nil {
		return f.createErr
	}
	if f.exists == nil {
		f.exists = make(map[string]bool)
	}
	f.exists[config.Name.ToCgroupfs()] = true
	return nil
}

func (f *fakePCMTestCgroupManager) Update(_ klog.Logger, _ *CgroupConfig) error {
	f.updateCalls++
	return f.updateErr
}

func (f *fakePCMTestCgroupManager) Destroy(_ klog.Logger, config *CgroupConfig) error {
	f.destroyCalls++
	if f.destroyErr != nil {
		return f.destroyErr
	}
	if f.exists != nil {
		delete(f.exists, config.Name.ToCgroupfs())
	}
	return nil
}

func (f *fakePCMTestCgroupManager) Validate(_ CgroupName) error { return nil }
func (f *fakePCMTestCgroupManager) Exists(name CgroupName) bool {
	if f.exists == nil {
		return false
	}
	return f.exists[name.ToCgroupfs()]
}
func (f *fakePCMTestCgroupManager) Name(name CgroupName) string { return name.ToCgroupfs() }
func (f *fakePCMTestCgroupManager) CgroupName(name string) CgroupName {
	return ParseCgroupfsToCgroupName(name)
}
func (f *fakePCMTestCgroupManager) Pids(_ klog.Logger, _ CgroupName) []int { return nil }
func (f *fakePCMTestCgroupManager) ReduceCPULimits(_ klog.Logger, _ CgroupName) error {
	return nil
}
func (f *fakePCMTestCgroupManager) MemoryUsage(_ CgroupName) (int64, error) { return 0, nil }
func (f *fakePCMTestCgroupManager) GetCgroupConfig(_ CgroupName, _ v1.ResourceName) (*ResourceConfig, error) {
	return nil, nil
}
func (f *fakePCMTestCgroupManager) SetCgroupConfig(_ klog.Logger, _ CgroupName, _ *ResourceConfig) error {
	return nil
}
func (f *fakePCMTestCgroupManager) Version() int { return 2 }

func TestEnsureExistsIdempotencyAndReconciliation(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	qosContainersInfo := QOSContainersInfo{
		Guaranteed: RootCgroupName,
		Burstable:  NewCgroupName(RootCgroupName, strings.ToLower(string(v1.PodQOSBurstable))),
		BestEffort: NewCgroupName(RootCgroupName, strings.ToLower(string(v1.PodQOSBestEffort))),
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("test-pod-reconcile"),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}

	t.Run("fresh pod creation succeeds and subsequent calls are no-op", func(t *testing.T) {
		fakeCM := &fakePCMTestCgroupManager{}
		pcm := &podContainerManagerImpl{
			cgroupManager:     fakeCM,
			qosContainersInfo: qosContainersInfo,
		}

		require.False(t, pcm.IsPodConfigured(pod))
		require.False(t, pcm.Exists(pod))

		// First call should Create
		err := pcm.EnsureExists(logger, pod)
		require.NoError(t, err)
		require.Equal(t, 1, fakeCM.createCalls)
		require.Equal(t, 0, fakeCM.updateCalls)
		require.True(t, pcm.IsPodConfigured(pod))
		require.True(t, pcm.Exists(pod))

		// Second call should be a no-op because it's already configured
		err = pcm.EnsureExists(logger, pod)
		require.NoError(t, err)
		require.Equal(t, 1, fakeCM.createCalls)
		require.Equal(t, 0, fakeCM.updateCalls)
	})

	t.Run("Create failure does not mark pod configured and retry calls Update when cgroup exists", func(t *testing.T) {
		fakeCM := &fakePCMTestCgroupManager{
			createErr: fmt.Errorf("cgroup manager.Set failed: dbus timeout"),
		}
		pcm := &podContainerManagerImpl{
			cgroupManager:     fakeCM,
			qosContainersInfo: qosContainersInfo,
		}

		// First call: Create fails
		err := pcm.EnsureExists(logger, pod)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cgroup manager.Set failed")
		require.Equal(t, 1, fakeCM.createCalls)
		require.Equal(t, 0, fakeCM.updateCalls)
		require.False(t, pcm.IsPodConfigured(pod))

		// In reality, Apply(-1) created the directory before Set failed, so Exists is now true
		fakeCM.createErr = nil
		fakeCM.exists = map[string]bool{
			NewCgroupName(qosContainersInfo.Burstable, "podtest-pod-reconcile").ToCgroupfs(): true,
		}
		require.True(t, pcm.Exists(pod))
		require.False(t, pcm.IsPodConfigured(pod))

		// Subsequent EnsureExists should call Update to reconcile
		err = pcm.EnsureExists(logger, pod)
		require.NoError(t, err)
		require.Equal(t, 1, fakeCM.createCalls) // Not called again
		require.Equal(t, 1, fakeCM.updateCalls) // Update was called to reconcile!
		require.True(t, pcm.IsPodConfigured(pod))

		// Further EnsureExists is a no-op
		err = pcm.EnsureExists(logger, pod)
		require.NoError(t, err)
		require.Equal(t, 1, fakeCM.updateCalls)
	})

	t.Run("kubelet restart reconciles existing pod via Update", func(t *testing.T) {
		fakeCM := &fakePCMTestCgroupManager{
			exists: map[string]bool{
				NewCgroupName(qosContainersInfo.Burstable, "podtest-pod-reconcile").ToCgroupfs(): true,
			},
		}
		// On restart, a new PCM starts with empty in-memory tracker
		pcm := &podContainerManagerImpl{
			cgroupManager:     fakeCM,
			qosContainersInfo: qosContainersInfo,
			podConfigTracker:  newPodConfigTracker(),
		}

		require.True(t, pcm.Exists(pod))
		require.False(t, pcm.IsPodConfigured(pod))

		// EnsureExists on restart runs Update once to reconcile and rebuilds in-memory state
		err := pcm.EnsureExists(logger, pod)
		require.NoError(t, err)
		require.Equal(t, 0, fakeCM.createCalls)
		require.Equal(t, 1, fakeCM.updateCalls)
		require.True(t, pcm.IsPodConfigured(pod))
	})

	t.Run("Destroy removes pod from in-memory tracker", func(t *testing.T) {
		fakeCM := &fakePCMTestCgroupManager{}
		pcm := &podContainerManagerImpl{
			cgroupManager:     fakeCM,
			qosContainersInfo: qosContainersInfo,
		}

		err := pcm.EnsureExists(logger, pod)
		require.NoError(t, err)
		require.True(t, pcm.IsPodConfigured(pod))

		podCgroupName, _ := pcm.GetPodContainerName(pod)
		err = pcm.Destroy(logger, podCgroupName)
		require.NoError(t, err)
		require.False(t, pcm.IsPodConfigured(pod))
	})
}
