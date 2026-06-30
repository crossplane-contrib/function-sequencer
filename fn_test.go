package main

import (
	"context"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/function-sequencer/input/v1beta1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	v1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

func TestRunFunction(t *testing.T) {
	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		mr    = `{"apiVersion":"example.org/v1","kind":"MR","metadata":{"name":"cool-mr"}}`
		nxr   = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr","namespace":"cool-namespace"},"spec":{"count":2}}`
		nmr   = `{"apiVersion":"example.org/v1","kind":"MR","metadata":{"name":"cool-mr","namespace":"cool-namespace"}}`
		uv1   = `{"apiVersion":"apiextensions.crossplane.io/v1beta1","kind":"Usage","metadata":{"name":"mr-cool-mr-xr-cool-xr-d9f469-dependency"},"spec":{"by":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"of":{"apiVersion":"example.org/v1","kind":"XR","resourceRef":{"name":"cool-xr"}},"reason":"dependency","replayDeletion":true}}`
		u2v1  = `{"apiVersion":"apiextensions.crossplane.io/v1beta1","kind":"Usage","metadata":{"name":"mr-cool-mr-mr-cool-mr-91201d-dependency"},"spec":{"by":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"of":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"reason":"dependency","replayDeletion":true}}`
		uv2   = `{"apiVersion":"protection.crossplane.io/v1beta1","kind":"ClusterUsage","metadata":{"name":"mr-cool-mr-xr-cool-xr-d9f469-dependency"},"spec":{"by":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"of":{"apiVersion":"example.org/v1","kind":"XR","resourceRef":{"name":"cool-xr"}},"reason":"dependency","replayDeletion":true}}`
		u2v2  = `{"apiVersion":"protection.crossplane.io/v1beta1","kind":"ClusterUsage","metadata":{"name":"mr-cool-mr-mr-cool-mr-91201d-dependency"},"spec":{"by":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"of":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"reason":"dependency","replayDeletion":true}}`
		nuv2  = `{"apiVersion":"protection.crossplane.io/v1beta1","kind":"Usage","metadata":{"name":"mr-cool-mr-xr-cool-xr-d9f469-dependency","namespace":"cool-namespace"},"spec":{"by":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"of":{"apiVersion":"example.org/v1","kind":"XR","resourceRef":{"name":"cool-xr"}},"reason":"dependency","replayDeletion":true}}`
		nu2v2 = `{"apiVersion":"protection.crossplane.io/v1beta1","kind":"Usage","metadata":{"name":"mr-cool-mr-mr-cool-mr-91201d-dependency","namespace":"cool-namespace"},"spec":{"by":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"of":{"apiVersion":"example.org/v1","kind":"MR","resourceRef":{"name":"cool-mr"}},"reason":"dependency","replayDeletion":true}}`
	)

	target := v1.Target_TARGET_COMPOSITE

	type args struct {
		ctx context.Context
		req *v1.RunFunctionRequest
	}
	type want struct {
		rsp *v1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ObservedAreSkipped": {
			reason: "Even though that second is skipped because it's in the observed state, delay third resource because first is not ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
									"third",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"third\" because \"first\" is not fully ready (0 of 1)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"FirstNotCreated": {
			reason: "The function should delay the creation of the second resource because the first not created",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"second\" because \"first\" does not exist yet",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
				},
			},
		},
		"FirstNotReady": {
			reason: "The function should delay the creation of the second resource because the first is not ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_FALSE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"second\" because \"first\" is not fully ready (0 of 1)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
		},
		"FirstReady": {
			reason: "The function should not delay the creation of the second resource because the first is ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"BothReady": {
			reason: "The function should return both of them",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"SequencesFirstNotReadyInBoth": {
			reason: "The function should delay the creation of second and fourth resources because the first and third are not ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
							{
								Sequence: []resource.Name{
									"third",
									"fourth",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
							},
							"fourth": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"second\" because \"first\" is not fully ready (0 of 1)",
							Target:   &target,
						},
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"fourth\" because \"third\" is not fully ready (0 of 1)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"SequencesFirstReadyInBoth": {
			reason: "The function should not delay the creation of any resource",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
							{
								Sequence: []resource.Name{
									"third",
									"fourth",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"fourth": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"fourth": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"OutOfSequence": {
			reason: "The function should delay the creation of second, but allow the creation of the other since its not in a sequence",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_FALSE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"outofsequence": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"second\" because \"first\" is not fully ready (0 of 1)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_FALSE,
							},
							"outofsequence": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"SequenceRegexNotAllReady": {
			reason: "The function should delay the creation of the second resource because the first-2 resource is not ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first-.*",
									"second",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"second\" because \"first-.*\" is not fully ready (2 of 3)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"SequenceRegexObservedAreSkipped": {
			reason: "The function should not attempt to remove resources from the desired state when they are already in the observed state",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first-.*",
									"second-.*",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"second-.*\" because \"first-.*\" is not fully ready (1 of 2)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"SequenceRegexFirstGroupReady": {
			reason: "The function should delay the creation of the third resource because the second-1 resource is not ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first-.*",
									"second-.*",
									"third",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"third\" because \"second-.*\" is not fully ready (1 of 2)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"MixedRegex": {
			reason: "The function should delay the creation of the third resource because the second-1 resource is not ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second-.*",
									"third",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"third\" because \"second-.*\" is not fully ready (1 of 2)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"SequenceRegexWaitComplex": {
			reason: "The function should not modify the sequence regex, since it's already prefixed",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first-.*",
									"second$",
									"third-resource",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_FALSE,
							},
							"0-second": {
								Resource: resource.MustStructJSON(mr),
							},
							"1-second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third-resource": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"second$\" because \"first-.*\" is not fully ready (1 of 2)",
							Target:   &target,
						},
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"third-resource\" because \"first-.*\" is not fully ready (1 of 2)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
		},
		"SequenceRegexAlreadyPrefixed": {
			reason: "The function should not modify the sequence regex, since it's already prefixed",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"^first-.*$",
									"^second-.*",
									"third-.*$",
									"fourth",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-0": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"fourth\" because \"third-.*$\" is not fully ready (0 of 1)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-0": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"SequenceRegexInvalidRegex": {
			reason: "The function should return a fatal error because the regex is invalid",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									`^(`,
									"second",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_FATAL,
							Message:  "cannot compile regex ^(: error parsing regexp: missing closing ): `^(`",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"FirstReadyUsageV1": {
			reason: "The function should create a V1 Usage when the first resource is ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
						UsageVersion: v1beta1.UsageV1,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-first-usage": {
								Resource: resource.MustStructJSON(uv1),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"FirstReadyUsageV2Cluster": {
			reason: "The function should create a V2 ClusterUsage when the first resource is ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
						UsageVersion: v1beta1.UsageV2,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-first-usage": {
								Resource: resource.MustStructJSON(uv2),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"FirstReadyUsageV2Namespaced": {
			reason: "The function should create a V2 Namespaced Usage when the first resource is ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
							},
						},
						UsageVersion: v1beta1.UsageV2,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(nxr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(nxr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(nxr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-first-usage": {
								Resource: resource.MustStructJSON(nuv2),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"MixedRegexUsageV1": {
			reason: "The function should create all resources and their V1 Usages because all desired resources are ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second-.*",
									"third",
								},
							},
						},
						UsageVersion: v1beta1.UsageV1,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0-first-usage": {
								Resource: resource.MustStructJSON(uv1),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1-first-usage": {
								Resource: resource.MustStructJSON(uv1),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-second-0-usage": {
								Resource: resource.MustStructJSON(u2v1),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-second-1-usage": {
								Resource: resource.MustStructJSON(u2v1),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"MixedRegexUsagePriorElementOnly": {
			reason: "The function should create all resources and Usages only for the prior element when regex is used",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
									"third-.*",
								},
							},
						},
						UsageVersion: v1beta1.UsageV1,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-first-usage": {
								Resource: resource.MustStructJSON(uv1),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-0-second-usage": {
								Resource: resource.MustStructJSON(u2v1),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-1-second-usage": {
								Resource: resource.MustStructJSON(u2v1),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"MixedRegexUsageV2Cluster": {
			reason: "The function should create all resources and Usages because all desired resources are ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second-.*",
									"third",
								},
							},
						},
						UsageVersion: v1beta1.UsageV2,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0-first-usage": {
								Resource: resource.MustStructJSON(uv2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1-first-usage": {
								Resource: resource.MustStructJSON(uv2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-second-0-usage": {
								Resource: resource.MustStructJSON(u2v2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-second-1-usage": {
								Resource: resource.MustStructJSON(u2v2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"MixedRegexUsageV2Namespaced": {
			reason: "The function should create all resources and Usages because all desired resources are ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second-.*",
									"third",
								},
							},
						},
						UsageVersion: v1beta1.UsageV2,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(nxr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(nxr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(nxr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-0-first-usage": {
								Resource: resource.MustStructJSON(nuv2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-1-first-usage": {
								Resource: resource.MustStructJSON(nuv2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-second-0-usage": {
								Resource: resource.MustStructJSON(nu2v2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third-second-1-usage": {
								Resource: resource.MustStructJSON(nu2v2),
								Ready:    v1.Ready_READY_TRUE,
							},
							"third": {
								Resource: resource.MustStructJSON(nmr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"DeletionSequencingBeforeResourceNotObserved": {
			reason: "The function should not panic when a before-resource is in desiredComposed but not in observedComposed with deletion sequencing enabled",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first-.*",
									"second-.*",
								},
							},
						},
						UsageVersion: v1beta1.UsageV2,
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"second-foo": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-foo": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-foo": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta:    &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first-foo": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-foo": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"MarkCompositeNotReady": {
			reason: "Set the Composite ready flag to false",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						ResetCompositeReadiness: true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
									"third",
								},
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource(s) matching \"third\" because \"first\" is not fully ready (0 of 1)",
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Ready:    v1.Ready_READY_FALSE,
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"ConditionTrueRunsNormally": {
			reason: "When condition evaluates to true, normal sequencing should apply — second blocked because first not ready",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: `observed.composite.resource.spec.count == 2.0`,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Delaying creation of resource(s) matching "second" because "first" is not fully ready (0 of 1)`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"ConditionDynBoolTrueRunsNormally": {
			reason: "When condition navigates into Struct fields (dyn type) and evaluates to true, normal sequencing should apply",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: `observed.composite.resource.spec.parameters.multiAZ == true`,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"parameters":{"multiAZ":true}}}`),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"parameters":{"multiAZ":true}}}`),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Delaying creation of resource(s) matching "second" because "first" is not fully ready (0 of 1)`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"parameters":{"multiAZ":true}}}`),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"ConditionDynBoolFalseSkipsSequence": {
			reason: "When condition navigates into Struct fields (dyn type) and evaluates to false, sequence should be skipped",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: `observed.composite.resource.spec.parameters.multiAZ == true`,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"parameters":{"multiAZ":false}}}`),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"parameters":{"multiAZ":false}}}`),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Skipping sequence [first second]: condition "observed.composite.resource.spec.parameters.multiAZ == true" evaluated to false`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"parameters":{"multiAZ":false}}}`),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"ConditionFalseSkipsSequence": {
			reason: "When condition evaluates to false, sequence should be skipped — second stays in desired and composite readiness is not reset even with resetCompositeReadiness=true",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						ResetCompositeReadiness: true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: `observed.composite.resource.spec.count == 999.0`,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Skipping sequence [first second]: condition "observed.composite.resource.spec.count == 999.0" evaluated to false`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"ConditionEmptyPassesThrough": {
			reason: "Empty condition string should behave identically to no condition — normal sequencing",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: "",
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Delaying creation of resource(s) matching "second" because "first" is not fully ready (0 of 1)`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"ConditionFalseProtectsObservedUsages": {
			reason: "When condition is false but resources are observed and deletion sequencing is enabled, Usages must still be generated",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: `observed.composite.resource.spec.count == 999.0`,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Skipping sequence [first second]: condition "observed.composite.resource.spec.count == 999.0" evaluated to false`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-first-usage": {
								Resource: resource.MustStructJSON(uv2),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"ConditionFalseNoObservedNoUsages": {
			reason: "When condition is false and no resources are observed, no Usages should be generated",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: `observed.composite.resource.spec.count == 999.0`,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Skipping sequence [first second]: condition "observed.composite.resource.spec.count == 999.0" evaluated to false`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"ConditionFalseNoDeletionSequencing": {
			reason: "When condition is false and enableDeletionSequencing is false, no Usages even with observed resources",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: false,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								Condition: `observed.composite.resource.spec.count == 999.0`,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_NORMAL,
							Message:  `Skipping sequence [first second]: condition "observed.composite.resource.spec.count == 999.0" evaluated to false`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"DeleteOnlySkipsCreationBlockingNoUsagesNoReadinessReset": {
			reason: "With deleteOnly=true and no deletion sequencing: resources are not blocked even when predecessors are not ready, no Usages are generated, and composite readiness is not reset even with resetCompositeReadiness=true",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						ResetCompositeReadiness: true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								DeleteOnly: true,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"CreateOnlyBlocksCreationButSkipsUsages": {
			reason: "createOnly=true still blocks creation when predecessors not ready, but generates no Usages even with enableDeletionSequencing=true",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence:   []resource.Name{"first", "second"},
								CreateOnly: true,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
						Resources: map[string]*v1.Resource{
							"first":  {Resource: resource.MustStructJSON(xr)},
							"second": {Resource: resource.MustStructJSON(mr)},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
						Resources: map[string]*v1.Resource{
							"first":  {Resource: resource.MustStructJSON(xr), Ready: v1.Ready_READY_TRUE},
							"second": {Resource: resource.MustStructJSON(mr), Ready: v1.Ready_READY_TRUE},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &v1.State{
						Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
						Resources: map[string]*v1.Resource{
							"first":  {Resource: resource.MustStructJSON(xr), Ready: v1.Ready_READY_TRUE},
							"second": {Resource: resource.MustStructJSON(mr), Ready: v1.Ready_READY_TRUE},
						},
					},
				},
			},
		},
		"CreateOnlyAndDeleteOnlyMutuallyExclusive": {
			reason: "Both createOnly and deleteOnly set returns fatal error",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						Rules: []v1beta1.SequencingRule{
							{
								Sequence:   []resource.Name{"first", "second"},
								CreateOnly: true,
								DeleteOnly: true,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
						Resources: map[string]*v1.Resource{},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
						Resources: map[string]*v1.Resource{
							"first":  {Resource: resource.MustStructJSON(mr)},
							"second": {Resource: resource.MustStructJSON(mr)},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*v1.Result{
						{
							Severity: v1.Severity_SEVERITY_FATAL,
							Message:  `rule for sequence [first second] cannot have both deleteOnly and createOnly set to true`,
							Target:   &target,
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
						Resources: map[string]*v1.Resource{
							"first":  {Resource: resource.MustStructJSON(mr)},
							"second": {Resource: resource.MustStructJSON(mr)},
						},
					},
				},
			},
		},
		"DeleteOnlyGeneratesUsages": {
			reason: "With deleteOnly=true and enableDeletionSequencing=true, Usage resources should still be created for observed resources",
			args: args{
				req: &v1.RunFunctionRequest{
					Input: resource.MustStructObject(&v1beta1.Input{
						EnableDeletionSequencing: true,
						ReplayDeletion:           true,
						Rules: []v1beta1.SequencingRule{
							{
								Sequence: []resource.Name{
									"first",
									"second",
								},
								DeleteOnly: true,
							},
						},
					}),
					Observed: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &v1.RunFunctionResponse{
					Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &v1.State{
						Composite: &v1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*v1.Resource{
							"first": {
								Resource: resource.MustStructJSON(xr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    v1.Ready_READY_TRUE,
							},
							"second-first-usage": {
								Resource: resource.MustStructJSON(uv2),
								Ready:    v1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunFunctionCacheTTL(t *testing.T) {
	target := v1.Target_TARGET_COMPOSITE
	xr := `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":1}}`

	cases := map[string]struct {
		reason string
		input  *v1beta1.Input
		want   *v1.RunFunctionResponse
	}{
		"ValidCacheTTL": {
			reason: "The function should override the response TTL when cacheTTL is provided",
			input: &v1beta1.Input{
				CacheTTL: "5m",
				Rules: []v1beta1.SequencingRule{
					{Sequence: []resource.Name{"first", "second"}},
				},
			},
			want: &v1.RunFunctionResponse{
				Meta: &v1.ResponseMeta{Ttl: durationpb.New(5 * time.Minute)},
				Results: []*v1.Result{
					{
						Severity: v1.Severity_SEVERITY_NORMAL,
						Message:  "Delaying creation of resource(s) matching \"second\" because \"first\" does not exist yet",
						Target:   &target,
					},
				},
				Desired: &v1.State{
					Composite: &v1.Resource{
						Resource: resource.MustStructJSON(xr),
					},
					Resources: map[string]*v1.Resource{},
				},
			},
		},
		"InvalidCacheTTL": {
			reason: "The function should return a fatal result when cacheTTL cannot be parsed",
			input: &v1beta1.Input{
				CacheTTL: "5x",
				Rules: []v1beta1.SequencingRule{
					{Sequence: []resource.Name{"first", "second"}},
				},
			},
			want: &v1.RunFunctionResponse{
				Meta: &v1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
				Results: []*v1.Result{
					{
						Severity: v1.Severity_SEVERITY_FATAL,
						Message:  "cannot set cacheTTL: time: unknown unit \"x\" in duration \"5x\"",
						Target:   &target,
					},
				},
				Desired: &v1.State{
					Composite: &v1.Resource{
						Resource: resource.MustStructJSON(xr),
					},
					Resources: map[string]*v1.Resource{
						"second": {
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			req := &v1.RunFunctionRequest{
				Input: resource.MustStructObject(tc.input),
				Observed: &v1.State{
					Composite: &v1.Resource{
						Resource: resource.MustStructJSON(xr),
					},
					Resources: map[string]*v1.Resource{},
				},
				Desired: &v1.State{
					Composite: &v1.Resource{
						Resource: resource.MustStructJSON(xr),
					},
					Resources: map[string]*v1.Resource{
						"second": {
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			}
			rsp, err := f.RunFunction(context.Background(), req)
			if diff := cmp.Diff(tc.want, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunFunctionConditionErrors(t *testing.T) {
	xr := `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
	mr := `{"apiVersion":"example.org/v1","kind":"MR","metadata":{"name":"cool-mr"}}`

	cases := map[string]struct {
		reason    string
		condition string
	}{
		"InvalidExpression": {
			reason:    "Invalid CEL syntax should produce Fatal",
			condition: `invalid $$$ expression`,
		},
		"NonBoolResult": {
			reason:    "CEL expression returning non-bool should produce Fatal",
			condition: `observed.composite.resource.spec.count`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			req := &v1.RunFunctionRequest{
				Input: resource.MustStructObject(&v1beta1.Input{
					Rules: []v1beta1.SequencingRule{
						{
							Sequence:  []resource.Name{"first", "second"},
							Condition: tc.condition,
						},
					},
				}),
				Observed: &v1.State{
					Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
					Resources: map[string]*v1.Resource{},
				},
				Desired: &v1.State{
					Composite: &v1.Resource{Resource: resource.MustStructJSON(xr)},
					Resources: map[string]*v1.Resource{
						"first":  {Resource: resource.MustStructJSON(mr)},
						"second": {Resource: resource.MustStructJSON(mr)},
					},
				},
			}
			rsp, err := f.RunFunction(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rsp.GetResults()) == 0 {
				t.Fatal("expected at least one result")
			}
			if rsp.GetResults()[0].GetSeverity() != v1.Severity_SEVERITY_FATAL {
				t.Errorf("%s: expected FATAL severity, got %s", tc.reason, rsp.GetResults()[0].GetSeverity())
			}
		})
	}
}
