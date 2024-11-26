package main

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/crossplane/function-sequencer/input/v1beta1"
)

func TestRunFunction(t *testing.T) {

	var (
		xr = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		mr = `{"apiVersion":"example.org/v1","kind":"MR","metadata":{"name":"cool-mr"}}`
	)

	type args struct {
		ctx context.Context
		req *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1beta1.RunFunctionResponse
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"third\" because \"first\" is not fully ready (0 of 1)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"second\" because \"first\" does not exist yet",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
				},
			},
		},
		"FirstNotReady": {
			reason: "The function should delay the creation of the second resource because the first is not ready",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_FALSE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"second\" because \"first\" is not fully ready (0 of 1)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
		},
		"FirstReady": {
			reason: "The function should not delay the creation of the second resource because the first is ready",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
		},
		"SequencesFirstNotReadyInBoth": {
			reason: "The function should delay the creation of second and fourth resources because the first and third are not ready",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"second\" because \"first\" is not fully ready (0 of 1)",
						},
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"fourth\" because \"third\" is not fully ready (0 of 1)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"fourth": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
							},
							"third": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_FALSE,
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"second\" because \"first\" is not fully ready (0 of 1)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_FALSE,
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"second\" because \"first-.*\" is not fully ready (2 of 3)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"SequenceRegexFirstGroupReady": {
			reason: "The function should delay the creation of second and fourth resources because the first and third are not ready",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"third\" because \"second-.*\" is not fully ready (1 of 2)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-2": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"third\" because \"second-.*\" is not fully ready (1 of 2)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second-1": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
		},
		"SequenceRegexAlreadyPrefixed": {
			reason: "The function should not modify the sequence regex, since it's already prefixed",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"third-0": {
								Resource: resource.MustStructJSON(mr),
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Delaying creation of resource \"fourth\" because \"third-.*$\" is not fully ready (0 of 1)",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
				req: &fnv1beta1.RunFunctionRequest{
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
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "cannot compile regex ^(: error parsing regexp: missing closing ): `^(`",
						},
					},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"first-0": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"first-1": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
							},
							"second": {
								Resource: resource.MustStructJSON(mr),
								Ready:    fnv1beta1.Ready_READY_TRUE,
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
