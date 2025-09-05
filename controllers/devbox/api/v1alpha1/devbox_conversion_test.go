/*
Copyright 2024.

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

package v1alpha1

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Test_transformCommitHistories(t *testing.T) {
	type args struct {
		commitHistories []*CommitHistory
		initialImage    string
	}
	// Test data based on the provided YAML
	time1, _ := time.Parse(time.RFC3339, "2024-12-04T08:32:50Z")
	time2, _ := time.Parse(time.RFC3339, "2024-12-23T03:44:46Z")
	time3, _ := time.Parse(time.RFC3339, "2025-01-09T19:32:07Z")

	tests := []struct {
		name  string
		args  args
		want  devboxv1alpha2.CommitRecordMap
		want1 string
	}{
		{
			name: "multiple commits ordered by time",
			args: args{
				commitHistories: []*CommitHistory{
					// Note: These are intentionally out of order to test sorting
					{
						ContainerID:      "containerd://e6013c9cfac677a4430bb1a3723bf34a0f1b9b5ea592da7e15b62ea40c336e65",
						Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446",
						Time:             metav1.NewTime(time2),
						Pod:              "chisel-r94ww",
						Status:           CommitStatusSuccess,
						PredicatedStatus: CommitStatusSuccess,
						Node:             "sealos-run-devbox-node0001",
					},
					{
						ContainerID:      "containerd://41af6b413371211697c360dc5f2b77c40543b3489c8f905ddfdb6bec9dcf2796",
						Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250",
						Time:             metav1.NewTime(time1),
						Pod:              "chisel-jx89l",
						Status:           CommitStatusSuccess,
						PredicatedStatus: CommitStatusSuccess,
						Node:             "sealos-run-devbox-node0002",
					},
					{
						ContainerID:      "containerd://a3665cf2c06b8aefcbdd767cbc6d67104e06ddad3a9bc58a65e6f55d6a45f5ff",
						Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:k4stc-2025-01-09-193207",
						Time:             metav1.NewTime(time3),
						Pod:              "chisel-l9k67",
						Status:           CommitStatusSuccess,
						PredicatedStatus: CommitStatusSuccess,
						Node:             "sealos-run-devbox-node0002",
					},
				},
				initialImage: "ghcr.io/labring-actions/devbox/go-1.23.0:13aacd8",
			},
			want: devboxv1alpha2.CommitRecordMap{
				// First commit (chronologically)
				"containerd://41af6b413371211697c360dc5f2b77c40543b3489c8f905ddfdb6bec9dcf2796": {
					BaseImage:    "ghcr.io/labring-actions/devbox/go-1.23.0:13aacd8",
					CommitImage:  "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250",
					Node:         "sealos-run-devbox-node0002",
					GenerateTime: metav1.NewTime(time1),
					ScheduleTime: metav1.NewTime(time1),
					UpdateTime:   metav1.NewTime(time1),
					CommitTime:   metav1.NewTime(time1),
					CommitStatus: devboxv1alpha2.CommitStatusSuccess,
				},
				// Second commit (uses first commit's image as base)
				"containerd://e6013c9cfac677a4430bb1a3723bf34a0f1b9b5ea592da7e15b62ea40c336e65": {
					BaseImage:    "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250",
					CommitImage:  "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446",
					Node:         "sealos-run-devbox-node0001",
					GenerateTime: metav1.NewTime(time2),
					ScheduleTime: metav1.NewTime(time2),
					UpdateTime:   metav1.NewTime(time2),
					CommitTime:   metav1.NewTime(time2),
					CommitStatus: devboxv1alpha2.CommitStatusSuccess,
				},
				// Third commit (uses second commit's image as base)
				"containerd://a3665cf2c06b8aefcbdd767cbc6d67104e06ddad3a9bc58a65e6f55d6a45f5ff": {
					BaseImage:    "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446",
					CommitImage:  "hub.hzh.sealos.run/ns-bekg6kgo/chisel:k4stc-2025-01-09-193207",
					Node:         "sealos-run-devbox-node0002",
					GenerateTime: metav1.NewTime(time3),
					ScheduleTime: metav1.NewTime(time3),
					UpdateTime:   metav1.NewTime(time3),
					CommitTime:   metav1.NewTime(time3),
					CommitStatus: devboxv1alpha2.CommitStatusSuccess,
				},
			},
			want1: "", // ContentID will be generated
		},
		{
			name: "commits with empty container ID should be skipped",
			args: args{
				commitHistories: []*CommitHistory{
					{
						ContainerID:      "", // Empty container ID
						Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250",
						Time:             metav1.NewTime(time1),
						Pod:              "chisel-jx89l",
						Status:           CommitStatusSuccess,
						PredicatedStatus: CommitStatusSuccess,
						Node:             "sealos-run-devbox-node0002",
					},
					{
						ContainerID:      "containerd://e6013c9cfac677a4430bb1a3723bf34a0f1b9b5ea592da7e15b62ea40c336e65",
						Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446",
						Time:             metav1.NewTime(time2),
						Pod:              "chisel-r94ww",
						Status:           CommitStatusSuccess,
						PredicatedStatus: CommitStatusSuccess,
						Node:             "sealos-run-devbox-node0001",
					},
				},
				initialImage: "ghcr.io/labring-actions/devbox/go-1.23.0:13aacd8",
			},
			want: devboxv1alpha2.CommitRecordMap{
				"containerd://e6013c9cfac677a4430bb1a3723bf34a0f1b9b5ea592da7e15b62ea40c336e65": {
					BaseImage:    "ghcr.io/labring-actions/devbox/go-1.23.0:13aacd8",
					CommitImage:  "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446",
					Node:         "sealos-run-devbox-node0001",
					GenerateTime: metav1.NewTime(time2),
					ScheduleTime: metav1.NewTime(time2),
					UpdateTime:   metav1.NewTime(time2),
					CommitTime:   metav1.NewTime(time2),
					CommitStatus: devboxv1alpha2.CommitStatusSuccess,
				},
			},
			want1: "", // ContentID will be generated
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := transformCommitHistories(tt.args.commitHistories, tt.args.initialImage)

			// print got to yaml
			gotYaml, _ := yaml.Marshal(got)
			fmt.Print(gotYaml)

			// For empty commit history, just check that contentID is generated and commitRecords is empty
			if len(tt.args.commitHistories) == 0 {
				if len(got) != 0 {
					t.Errorf("transformCommitHistories() got %d records, want 0 for empty input", len(got))
				}
				if got1 == "" {
					t.Errorf("transformCommitHistories() contentID should not be empty for empty input")
				}
				return
			}

			// For non-empty commit history, check that we have the expected records plus one additional contentID record
			expectedCount := len(tt.want) + 1 // +1 for the contentID record
			if len(got) != expectedCount {
				t.Errorf("transformCommitHistories() got %d records, want %d", len(got), expectedCount)
			}

			// Check that contentID is not empty and exists in the map
			if got1 == "" {
				t.Errorf("transformCommitHistories() contentID should not be empty")
			}

			if _, exists := got[got1]; !exists {
				t.Errorf("transformCommitHistories() contentID %s should exist in commit records", got1)
			}

			// Check each expected record (excluding the contentID record)
			for key, expectedRecord := range tt.want {
				actualRecord, exists := got[key]
				if !exists {
					t.Errorf("transformCommitHistories() missing expected record with key %s", key)
					continue
				}

				if !reflect.DeepEqual(actualRecord, expectedRecord) {
					t.Errorf("transformCommitHistories() record %s = %v, want %v", key, actualRecord, expectedRecord)
				}
			}

			// Check the contentID record properties
			contentIDRecord := got[got1]
			if contentIDRecord == nil {
				t.Errorf("transformCommitHistories() contentID record should not be nil")
			} else {
				if contentIDRecord.CommitStatus != devboxv1alpha2.CommitStatusPending {
					t.Errorf("transformCommitHistories() contentID record status = %v, want %v",
						contentIDRecord.CommitStatus, devboxv1alpha2.CommitStatusPending)
				}
				if contentIDRecord.CommitImage != "" {
					t.Errorf("transformCommitHistories() contentID record CommitImage should be empty, got %s",
						contentIDRecord.CommitImage)
				}
				// BaseImage should be the last commit's image
				if len(tt.args.commitHistories) > 0 {
					// Find the latest commit by time
					var latestCommit *CommitHistory
					for _, commit := range tt.args.commitHistories {
						if commit.ContainerID == "" {
							continue
						}
						if latestCommit == nil || commit.Time.Time.After(latestCommit.Time.Time) {
							latestCommit = commit
						}
					}
					if latestCommit != nil && contentIDRecord.BaseImage != latestCommit.Image {
						t.Errorf("transformCommitHistories() contentID record BaseImage = %s, want %s",
							contentIDRecord.BaseImage, latestCommit.Image)
					}
				}
			}
		})
	}
}

func TestDevbox_ConvertTo(t *testing.T) {
	// Test data based on the provided YAML
	time1, _ := time.Parse(time.RFC3339, "2024-12-04T08:32:50Z")
	time2, _ := time.Parse(time.RFC3339, "2024-12-23T03:44:46Z")
	time3, _ := time.Parse(time.RFC3339, "2025-01-09T19:32:07Z")
	creationTime, _ := time.Parse(time.RFC3339, "2024-12-04T08:32:50Z")

	src := &Devbox{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "devbox.sealos.io/v1alpha1",
			Kind:       "Devbox",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "chisel",
			Namespace:         "ns-bekg6kgo",
			CreationTimestamp: metav1.NewTime(creationTime),
			Finalizers:        []string{"devbox.sealos.io/finalizer"},
			Generation:        9,
			Labels: map[string]string{
				"devbox.sealos.io/patched": "true",
			},
			ResourceVersion: "2318032405",
			UID:             "cce6da78-8b67-4cc2-bd03-cabe6e537487",
		},
		Spec: DevboxSpec{
			State: DevboxStateStopped,
			Resource: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Image:      "ghcr.io/labring-actions/devbox/go-1.23.0:13aacd8",
			TemplateID: "01937107-4f89-4885-8083-91bcda92cf99",
			Squash:     false,
			Config: Config{
				User:       "devbox",
				WorkingDir: "/home/devbox/project",
				AppPorts: []corev1.ServicePort{
					{
						Name:       "devbox-app-port",
						Port:       8080,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(0),
					},
				},
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 22,
						Name:          "devbox-ssh-port",
						Protocol:      corev1.ProtocolTCP,
					},
				},
				ReleaseCommand: []string{"/bin/bash", "-c"},
				ReleaseArgs:    []string{"/home/devbox/project/entrypoint.sh"},
			},
			NetworkSpec: NetworkSpec{
				Type: NetworkTypeNodePort,
			},
		},
		Status: DevboxStatus{
			Phase: DevboxPhaseStopped,
			Network: NetworkStatus{
				Type:     NetworkTypeNodePort,
				NodePort: 36286,
				TailNet:  "",
			},
			CommitHistory: []*CommitHistory{
				{
					ContainerID:      "containerd://41af6b413371211697c360dc5f2b77c40543b3489c8f905ddfdb6bec9dcf2796",
					Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250",
					Time:             metav1.NewTime(time1),
					Pod:              "chisel-jx89l",
					Status:           CommitStatusSuccess,
					PredicatedStatus: CommitStatusSuccess,
					Node:             "sealos-run-devbox-node0002",
				},
				{
					ContainerID:      "containerd://e6013c9cfac677a4430bb1a3723bf34a0f1b9b5ea592da7e15b62ea40c336e65",
					Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446",
					Time:             metav1.NewTime(time2),
					Pod:              "chisel-r94ww",
					Status:           CommitStatusSuccess,
					PredicatedStatus: CommitStatusSuccess,
					Node:             "sealos-run-devbox-node0001",
				},
				{
					ContainerID:      "containerd://a3665cf2c06b8aefcbdd767cbc6d67104e06ddad3a9bc58a65e6f55d6a45f5ff",
					Image:            "hub.hzh.sealos.run/ns-bekg6kgo/chisel:k4stc-2025-01-09-193207",
					Time:             metav1.NewTime(time3),
					Pod:              "chisel-l9k67",
					Status:           CommitStatusSuccess,
					PredicatedStatus: CommitStatusSuccess,
					Node:             "sealos-run-devbox-node0002",
				},
			},
		},
	}

	dst := &devboxv1alpha2.Devbox{}
	err := src.ConvertTo(dst)

	if err != nil {
		t.Errorf("ConvertTo() error = %v", err)
		return
	}

	// Check basic fields
	if dst.APIVersion != "devbox.sealos.io/v1alpha2" {
		t.Errorf("ConvertTo() APIVersion = %v, want %v", dst.APIVersion, "devbox.sealos.io/v1alpha2")
	}

	if dst.Kind != "Devbox" {
		t.Errorf("ConvertTo() Kind = %v, want %v", dst.Kind, "Devbox")
	}

	if dst.Name != "chisel" {
		t.Errorf("ConvertTo() Name = %v, want %v", dst.Name, "chisel")
	}

	if dst.Namespace != "ns-bekg6kgo" {
		t.Errorf("ConvertTo() Namespace = %v, want %v", dst.Namespace, "ns-bekg6kgo")
	}

	// Check that we have 4 commit records (3 original + 1 contentID)
	if len(dst.Status.CommitRecords) != 4 {
		t.Errorf("ConvertTo() CommitRecords count = %v, want %v", len(dst.Status.CommitRecords), 4)
	}

	// Check that ContentID is set and exists in CommitRecords
	if dst.Status.ContentID == "" {
		t.Errorf("ConvertTo() ContentID should not be empty")
	}

	if _, exists := dst.Status.CommitRecords[dst.Status.ContentID]; !exists {
		t.Errorf("ConvertTo() ContentID %s should exist in CommitRecords", dst.Status.ContentID)
	}

	// Check the commit chain - verify that baseImage -> commitImage relationships are correct
	// First commit should have initial image as base
	firstCommitRecord := dst.Status.CommitRecords["containerd://41af6b413371211697c360dc5f2b77c40543b3489c8f905ddfdb6bec9dcf2796"]
	if firstCommitRecord == nil {
		t.Errorf("ConvertTo() first commit record should exist")
	} else {
		if firstCommitRecord.BaseImage != "ghcr.io/labring-actions/devbox/go-1.23.0:13aacd8" {
			t.Errorf("ConvertTo() first commit BaseImage = %v, want %v",
				firstCommitRecord.BaseImage, "ghcr.io/labring-actions/devbox/go-1.23.0:13aacd8")
		}
		if firstCommitRecord.CommitImage != "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250" {
			t.Errorf("ConvertTo() first commit CommitImage = %v, want %v",
				firstCommitRecord.CommitImage, "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250")
		}
	}

	// Second commit should have first commit's image as base
	secondCommitRecord := dst.Status.CommitRecords["containerd://e6013c9cfac677a4430bb1a3723bf34a0f1b9b5ea592da7e15b62ea40c336e65"]
	if secondCommitRecord == nil {
		t.Errorf("ConvertTo() second commit record should exist")
	} else {
		if secondCommitRecord.BaseImage != "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250" {
			t.Errorf("ConvertTo() second commit BaseImage = %v, want %v",
				secondCommitRecord.BaseImage, "hub.hzh.sealos.run/ns-bekg6kgo/chisel:8992k-2024-12-04-083250")
		}
	}

	// Third commit should have second commit's image as base
	thirdCommitRecord := dst.Status.CommitRecords["containerd://a3665cf2c06b8aefcbdd767cbc6d67104e06ddad3a9bc58a65e6f55d6a45f5ff"]
	if thirdCommitRecord == nil {
		t.Errorf("ConvertTo() third commit record should exist")
	} else {
		if thirdCommitRecord.BaseImage != "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446" {
			t.Errorf("ConvertTo() third commit BaseImage = %v, want %v",
				thirdCommitRecord.BaseImage, "hub.hzh.sealos.run/ns-bekg6kgo/chisel:5ck9z-2024-12-23-034446")
		}
	}

	// ContentID record should have the last commit's image as base
	contentIDRecord := dst.Status.CommitRecords[dst.Status.ContentID]
	if contentIDRecord == nil {
		t.Errorf("ConvertTo() contentID record should exist")
	} else {
		if contentIDRecord.BaseImage != "hub.hzh.sealos.run/ns-bekg6kgo/chisel:k4stc-2025-01-09-193207" {
			t.Errorf("ConvertTo() contentID record BaseImage = %v, want %v",
				contentIDRecord.BaseImage, "hub.hzh.sealos.run/ns-bekg6kgo/chisel:k4stc-2025-01-09-193207")
		}
		if contentIDRecord.CommitStatus != devboxv1alpha2.CommitStatusPending {
			t.Errorf("ConvertTo() contentID record CommitStatus = %v, want %v",
				contentIDRecord.CommitStatus, devboxv1alpha2.CommitStatusPending)
		}
	}
}
