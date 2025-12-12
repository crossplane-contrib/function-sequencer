package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/function-sequencer/input/v1beta1"

	v1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

// Function returns whatever response you ask it to.
type Function struct {
	v1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

const (
	// START marks the start of a regex pattern.
	START = "^"
	// END marks the end of a regex pattern.
	END = "$"
)

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *v1.RunFunctionRequest) (*v1.RunFunctionResponse, error) { //nolint:gocognit // This function is unavoidably complex.
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
				beforeRegex, err := getStrictRegex(string(before))
				if err != nil {
					response.Fatal(rsp, errors.Wrapf(err, "cannot compile regex %s", before))
					return rsp, nil
				}
				keys := []resource.Name{}
				for k := range desiredComposed {
					if beforeRegex.MatchString(string(k)) {
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
					msg := fmt.Sprintf("Delaying creation of resource(s) matching %q because %q does not exist yet", r, before)
					if desired > 0 {
						// provide a nicer message if there are resources.
						msg = fmt.Sprintf(
							"Delaying creation of resource(s) matching %q because %q is not fully ready (%d of %d)",
							r,
							before,
							readyResources,
							desired,
						)
					}
					response.Normal(rsp, msg)
					f.log.Info(msg)
					// find all objects that match the regex and delete them from the desiredComposed map
					currentRegex, _ := getStrictRegex(string(r))
					for k := range desiredComposed {
						if currentRegex.MatchString(string(k)) {
							if _, ok := observedComposed[k]; ok {
								// if the resource is already part of the observedComposed, we should not delete it
								continue
							}
							delete(desiredComposed, k)
							if in.ResetCompositeReadiness {
								// Reset the composite ready indicator to false when a desired resource is deleted.
								rsp.Desired.Composite.Ready = v1.Ready_READY_FALSE
							}
						}
					}
					break
				}
			}
		}
	}

	rsp.Desired.Resources = nil
	return rsp, response.SetDesiredComposedResources(rsp, desiredComposed)
}

func getStrictRegex(pattern string) (*regexp.Regexp, error) {
	if !strings.HasPrefix(pattern, START) && !strings.HasSuffix(pattern, END) {
		// if the user provides a delimited regex, we'll use it as is
		// if not, add the regex with ^ & $ to match the entire string
		// possibly avoid using regex for matching literal strings
		pattern = fmt.Sprintf("%s%s%s", START, pattern, END)
	}
	return regexp.Compile(pattern)
}
