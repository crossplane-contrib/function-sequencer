package main

import (
	"context"
	"testing"

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
		xr = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		mr = `{"apiVersion":"example.org/v1","kind":"MR","metadata":{"name":"cool-mr"}}`
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
			reason: "The function should delay the creation of second and fourth resources because the first and third are not ready",
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
			reason: "The function should delay the creation of second and fourth resources because the first and third are not ready",
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
			reason: "The function should delay the creation of second and fourth resources because the first and third are not ready",
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
