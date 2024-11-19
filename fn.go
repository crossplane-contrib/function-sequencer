package main

import (
	"context"
	"fmt"
	"regexp"

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
				re := regexp.MustCompile(string(before))
				keys := []resource.Name{}
				for k := range desiredComposed {
					if re.MatchString(string(k)) {
						keys = append(keys, k)
					}
				}

				// We'll treat everything the same way adding all resources to the keys slice
				// and then checking if they are ready.
				desired := len(keys)
				readyResources := 0
				for _, k := range keys {
					if b, ok := desiredComposed[k]; ok && b.Ready == resource.ReadyTrue {
						// resource is ready, add it to the counter
						readyResources++
					}
				}

				if desired == 0 || desired != readyResources {
					// no resource created
					msg := fmt.Sprintf("Delaying creation of resource %q because %q does not exist yet", r, before)
					if desired > 0 {
						// provide a nicer message if there are resources.
						msg = fmt.Sprintf(
							"Delaying creation of resource %q because %q is not fully ready (%d of %d)",
							r,
							before,
							readyResources,
							desired,
						)
					}
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
