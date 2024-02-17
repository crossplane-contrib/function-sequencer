package main

import (
	"context"
	"github.com/crossplane/function-sdk-go/resource"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/crossplane/function-sequencer/input/v1beta1"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	response.Normalf(rsp, "I was run with input %q!", in.Sequence)
	f.log.Info("I was run!", "input", in.Sequence)

	//  Get the desired composed resources from the request.
	desiredComposed, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get desired composed resources"))
		return rsp, nil
	}

	observedComposed, err := request.GetObservedComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composed resources"))
		return rsp, nil
	}

	for i, r := range in.Sequence {
		if i == 0 {
			// We don't need to do anything for the first resource in the sequence.
			continue
		}
		if _, created := observedComposed[r]; created {
			// We've already created this resource, so we don't need to do anything.
			// We only sequence creation of resources that don't exist yet.
			continue
		}
		for _, before := range in.Sequence[:i] {
			if b, ok := desiredComposed[before]; !ok || b.Ready != resource.ReadyTrue {
				// A resource that should exist before this one is not in the desired list, or it is not ready yet.
				// So, we should not create the resource waiting for it yet.
				delete(desiredComposed, r)
				break
			}
		}
	}

	rsp.Desired.Resources = nil
	return rsp, response.SetDesiredComposedResources(rsp, desiredComposed)
}
