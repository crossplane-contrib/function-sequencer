package main

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/crossplane/function-sequencer/input/v1beta1"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) { //nolint:gocyclo // This function is unavoidably complex.
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

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

	sequences := make([][]resource.Name, 0, len(in.Rules))
	for _, rule := range in.Rules {
		sequences = append(sequences, rule.Sequence)
	}

	for _, sequence := range sequences {
		for i, r := range sequence {
			if i == 0 {
				// We don't need to do anything for the first resource in the sequence.
				continue
			}
			if _, created := observedComposed[r]; created {
				// We've already created this resource, so we don't need to do anything.
				// We only sequence creation of resources that don't exist yet.
				continue
			}
			for _, before := range sequence[:i] {
				if b, ok := desiredComposed[before]; !ok || b.Ready != resource.ReadyTrue {
					// A resource that should exist before this one is not in the desired list, or it is not ready yet.
					// So, we should not create the resource waiting for it yet.
					msg := fmt.Sprintf("Delaying creation of resource %q because %q is not ready or does not exist yet", r, before)
					response.Normal(rsp, msg)
					f.log.Info(msg)
					delete(desiredComposed, r)
					break
				}
			}
		}
	}

	rsp.Desired.Resources = nil
	return rsp, response.SetDesiredComposedResources(rsp, desiredComposed)
}
