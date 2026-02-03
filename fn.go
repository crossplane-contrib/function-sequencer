package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	apiextensionsv1beta1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1beta1"
	protectionv1beta1 "github.com/crossplane/crossplane/v2/apis/protection/v1beta1"
	"github.com/crossplane/function-sequencer/input/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
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

const (
	DependencyReason         = "dependency"
	ProtectionGroupVersion   = protectionv1beta1.Group + "/" + protectionv1beta1.Version
	ProtectionV1GroupVersion = apiextensionsv1beta1.Group + "/" + apiextensionsv1beta1.Version
	// UsageNameSuffix is the suffix applied when generating Usage names.
	UsageNameSuffix = "dependency"
	// V1ModeError Error when trying to protect a namespaced resource when in v1 mode.
	V1ModeError = "cannot protect namespaced resource (kind: %s, name: %s, namespace: %s) with enableV1Mode=true. v1 usages only support cluster-scoped resources."
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
	usages := make(map[resource.Name]*resource.DesiredComposed)

	for _, sequence := range sequences {
		for i, r := range sequence {
			if i == 0 {
				// We don't need to do anything for the first resource in the sequence.
				continue
			}
			if _, created := observedComposed[r]; created {
				f.log.Debug("Processing ", "r:", r)
				if in.EnableDeletionSequencing {
					of := sequence[i-1]
					ofRegex, err := getStrictRegex(string(of))
					if err != nil {
						response.Fatal(rsp, errors.Wrapf(err, "cannot compile regex %s", of))
						return rsp, nil
					}
					for k := range desiredComposed {
						if ofRegex.MatchString(string(k)) {
							if _, ok := observedComposed[k]; ok {
								f.log.Debug("Generate Usage", "of:", k, "by r:", r)
								usage := GenerateUsage(&observedComposed[k].Resource.Unstructured, &observedComposed[r].Resource.Unstructured, in.ReplayDeletion, in.UsageVersion)
								usageComposed := composed.New()
								if err := convertViaJSON(usageComposed, usage); err != nil {
									response.Fatal(rsp, errors.Wrapf(err, "cannot convert to JSON %s", usage))
									return rsp, err
								}
								f.log.Debug("created usage", "kind", usageComposed.GetKind(), "name", usageComposed.GetName(), "namespace", usageComposed.GetNamespace())
								usages[r+"-"+k+"-usage"] = &resource.DesiredComposed{Resource: usageComposed, Ready: resource.ReadyTrue}
							}
						}
					}
				}
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

				currentRegex, err := getStrictRegex(string(r))
				if err != nil {
					response.Fatal(rsp, errors.Wrapf(err, "cannot compile regex %s", r))
					return rsp, nil
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
				if in.EnableDeletionSequencing {
					for c, o := range observedComposed {
						if currentRegex.MatchString(string(c)) && !isUsage(o, in.UsageVersion) {
							for _, k := range keys {
								f.log.Debug("Generate Usage of ", "k:", k, "by c:", c)
								usage := GenerateUsage(&observedComposed[k].Resource.Unstructured, &o.Resource.Unstructured, in.ReplayDeletion, in.UsageVersion)
								usageComposed := composed.New()
								if err := convertViaJSON(usageComposed, usage); err != nil {
									response.Fatal(rsp, errors.Wrapf(err, "cannot convert to JSON %s", usage))
									return rsp, err
								}
								f.log.Debug("created usage", "kind", usageComposed.GetKind(), "name", usageComposed.GetName(), "namespace", usageComposed.GetNamespace())
								usages[c+"-"+k+"-usage"] = &resource.DesiredComposed{Resource: usageComposed, Ready: resource.ReadyTrue}
							}
						}
					}
				}
			}
		}
	}
	maps.Copy(desiredComposed, usages)
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

// GenerateUsage determines whether to return a v1 or v2 Crossplane usage.
func GenerateUsage(of, by *unstructured.Unstructured, rd bool, usageVersion v1beta1.UsageVersion) map[string]any {
	if usageVersion == v1beta1.UsageV1 {
		return GenerateV1Usage(of, by, rd)
	}
	return GenerateV2Usage(of, by, rd)
}

// GenerateV2Usage creates a v2 Usage for a resource.
func GenerateV2Usage(of *unstructured.Unstructured, by *unstructured.Unstructured, rd bool) map[string]any {
	name := strings.ToLower(by.GetKind() + "-" + by.GetName() + "-" + of.GetKind() + "-" + of.GetName())
	usageType := protectionv1beta1.ClusterUsageKind
	usageMeta := map[string]any{
		"name": GenerateName(name, UsageNameSuffix),
	}

	namespace := of.GetNamespace()
	if namespace != "" {
		usageType = protectionv1beta1.UsageKind
		usageMeta["namespace"] = namespace
	}

	usage := map[string]any{
		"apiVersion": ProtectionGroupVersion,
		"kind":       usageType,
		"metadata":   usageMeta,
		"spec": map[string]any{
			"by": map[string]any{
				"apiVersion": by.GetAPIVersion(),
				"kind":       by.GetKind(),
				"resourceRef": map[string]any{
					"name": by.GetName(),
				},
			},
			"of": map[string]any{
				"apiVersion": of.GetAPIVersion(),
				"kind":       of.GetKind(),
				"resourceRef": map[string]any{
					"name": of.GetName(),
				},
			},
			"reason":         DependencyReason,
			"replayDeletion": rd,
		},
	}
	return usage
}

// GenerateV1Usage creates a Crossplane v1 Usage for a resource.
// Only Cluster Scoped Resources are supported.
func GenerateV1Usage(of *unstructured.Unstructured, by *unstructured.Unstructured, rd bool) map[string]any {
	name := strings.ToLower(by.GetKind() + "-" + by.GetName() + "-" + of.GetKind() + "-" + of.GetName())
	usage := map[string]any{
		"apiVersion": ProtectionV1GroupVersion,
		"kind":       apiextensionsv1beta1.UsageKind,
		"metadata": map[string]any{
			"name": GenerateName(name, UsageNameSuffix),
		},
		"spec": map[string]any{
			"by": map[string]any{
				"apiVersion": by.GetAPIVersion(),
				"kind":       by.GetKind(),
				"resourceRef": map[string]any{
					"name": by.GetName(),
				},
			},
			"of": map[string]any{
				"apiVersion": of.GetAPIVersion(),
				"kind":       of.GetKind(),
				"resourceRef": map[string]any{
					"name": of.GetName(),
				},
			},
			"reason":         DependencyReason,
			"replayDeletion": rd,
		},
	}
	return usage
}

func convertViaJSON(to, from any) error {
	bs, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, to)
}

func isUsage(composed resource.ObservedComposed, usageVersion v1beta1.UsageVersion) bool {
	kind := composed.Resource.GetKind()
	if usageVersion == v1beta1.UsageV1 {
		return kind == apiextensionsv1beta1.UsageKind
	}
	return kind == protectionv1beta1.ClusterUsageKind || kind == protectionv1beta1.UsageKind
}
