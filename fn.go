package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	apiextensionsv1beta1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1beta1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
	"github.com/crossplane/function-sequencer/input/v1beta1"
	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
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

// celEnv lazily initializes the shared CEL environment on first use.
var celEnv = sync.OnceValues(func() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Types(&v1.State{}, &structpb.Struct{}),
		cel.Variable("observed", cel.ObjectType("apiextensions.fn.proto.v1.State")),
		cel.Variable("desired", cel.ObjectType("apiextensions.fn.proto.v1.State")),
		cel.Variable("context", cel.ObjectType("google.protobuf.Struct")),
	)
})

// evaluateCondition evaluates a CEL expression against the function request.
func (f *Function) evaluateCondition(req *v1.RunFunctionRequest, condition string) (bool, error) {
	env, err := celEnv()
	if err != nil {
		return false, errors.Wrap(err, "cannot create CEL environment")
	}
	ast, iss := env.Parse(condition)
	if iss.Err() != nil {
		return false, errors.Wrap(iss.Err(), "cannot parse CEL condition")
	}
	checked, iss := env.Check(ast)
	if iss.Err() != nil {
		return false, errors.Wrap(iss.Err(), "cannot type-check CEL condition")
	}
	if !checked.OutputType().IsExactType(cel.BoolType) {
		return false, errors.Errorf("CEL condition must return bool, got %s", checked.OutputType())
	}
	program, err := env.Program(checked)
	if err != nil {
		return false, errors.Wrap(err, "cannot compile CEL condition")
	}
	result, _, err := program.Eval(map[string]any{
		"observed": req.GetObserved(),
		"desired":  req.GetDesired(),
		"context":  req.GetContext(),
	})
	if err != nil {
		return false, errors.Wrap(err, "cannot evaluate CEL condition")
	}
	ret, ok := result.Value().(bool)
	if !ok {
		return false, errors.New("CEL condition result is not bool")
	}
	return ret, nil
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
	f.log.Debug("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}
	if in.CacheTTL != "" {
		dur, err := time.ParseDuration(in.CacheTTL)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot set cacheTTL"))
			return rsp, nil
		}
		rsp.Meta.Ttl = durationpb.New(dur)
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

	usages := make(map[resource.Name]*resource.DesiredComposed)

	for _, rule := range in.Rules {
		sequence := rule.Sequence

		if rule.Condition != "" {
			conditionMet, err := f.evaluateCondition(req, rule.Condition)
			if err != nil {
				response.Fatal(rsp, errors.Wrapf(err, "cannot evaluate condition %q for sequence %v", rule.Condition, sequence))
				return rsp, nil
			}
			if !conditionMet {
				f.log.Debug("Skipping sequence due to false condition", "condition", rule.Condition, "sequence", sequence)
				response.Normal(rsp, fmt.Sprintf("Skipping sequence %v: condition %q evaluated to false", sequence, rule.Condition))
				if in.EnableDeletionSequencing {
					if err := f.generateObservedUsages(sequence, observedComposed, desiredComposed, usages, in.ReplayDeletion, in.UsageVersion); err != nil {
						response.Fatal(rsp, errors.Wrap(err, "cannot generate usages for skipped sequence"))
						return rsp, err
					}
				}
				continue
			}
		}

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
			if rule.DeleteOnly {
				continue
			}
			for b, before := range sequence[:i] {
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
					if d, ok := desiredComposed[k]; ok && d.Ready == resource.ReadyTrue {
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
					f.log.Debug(msg)
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
				// Only create Usages of the previous (i-1) resource in the sequence.
				if b == i-1 && in.EnableDeletionSequencing {
					for c, o := range observedComposed {
						if currentRegex.MatchString(string(c)) && !isUsage(o, in.UsageVersion) {
							for _, k := range keys {
								obs, ok := observedComposed[k]
								if !ok {
									f.log.Debug("Skipping usage; before-resource not yet observed", "k:", k, "by c:", c)
									continue
								}
								f.log.Debug("Generate Usage of ", "k:", k, "by c:", c)
								usage := GenerateUsage(&obs.Resource.Unstructured, &o.Resource.Unstructured, in.ReplayDeletion, in.UsageVersion)
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

// generateObservedUsages creates Usage/ClusterUsage resources for observed resources in a sequence,
// protecting deletion order even when the sequence is skipped (e.g. condition evaluates to false).
func (f *Function) generateObservedUsages(
	sequence []resource.Name,
	observedComposed map[resource.Name]resource.ObservedComposed,
	desiredComposed map[resource.Name]*resource.DesiredComposed,
	usages map[resource.Name]*resource.DesiredComposed,
	replayDeletion bool,
	usageVersion v1beta1.UsageVersion,
) error {
	for i := 1; i < len(sequence); i++ {
		rRegex, err := getStrictRegex(string(sequence[i]))
		if err != nil {
			return errors.Wrapf(err, "cannot compile regex %s", sequence[i])
		}
		ofRegex, err := getStrictRegex(string(sequence[i-1]))
		if err != nil {
			return errors.Wrapf(err, "cannot compile regex %s", sequence[i-1])
		}
		for c, o := range observedComposed {
			if !rRegex.MatchString(string(c)) || isUsage(o, usageVersion) {
				continue
			}
			for k := range desiredComposed {
				if !ofRegex.MatchString(string(k)) {
					continue
				}
				if obs, ok := observedComposed[k]; ok {
					f.log.Debug("Generate Usage for observed resource", "of:", k, "by:", c)
					usage := GenerateUsage(&obs.Resource.Unstructured, &o.Resource.Unstructured, replayDeletion, usageVersion)
					usageComposed := composed.New()
					if err := convertViaJSON(usageComposed, usage); err != nil {
						return errors.Wrapf(err, "cannot convert to JSON %s", usage)
					}
					usages[c+"-"+k+"-usage"] = &resource.DesiredComposed{Resource: usageComposed, Ready: resource.ReadyTrue}
				}
			}
		}
	}
	return nil
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
	apiVersion := composed.Resource.GetAPIVersion()
	kind := composed.Resource.GetKind()
	if usageVersion == v1beta1.UsageV1 {
		return apiVersion == ProtectionV1GroupVersion && kind == apiextensionsv1beta1.UsageKind
	}
	return apiVersion == ProtectionGroupVersion && (kind == protectionv1beta1.ClusterUsageKind || kind == protectionv1beta1.UsageKind)
}
