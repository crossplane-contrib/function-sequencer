# function-sequencer

Function Sequencer is a Crossplane function that enables Composition authors to define sequencing rules delaying the
creation of resources until other resources are ready.  The same sequencing rules can be used, in reverse, to define the order that resources are deleted, when foreground cascading deletion is used.

For example, the pipeline step below will ensure that `second-resource` and `third-resource` not to be created until
the `first-resource` is ready.

```yaml
  - step: sequence-creation
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      rules:
        - sequence:
          - first-resource
          - second-resource
        - sequence:
          - first-resource
          - third-resource
```

See `example/composition.yaml` for a complete example.


It also allows you to provide a regex to match and capture a whole group of resources and wait for them to be ready.

For example, the pipeline step below, will ensure that `second-resource` is not created until all `first-subresource-.*` objects are ready.

```yaml
  - step: sequence-creation
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      rules:
        - sequence:
          - first-subresource-.*
          - second-resource
```

You can write the regex as strict as you want, but keep in mind that it defaults to strict matching (start and end are enforced).
In other words, the following rules apply:
```yaml
- resource    # this has no explicit start or end, so it will match EXACTLY ^resource$ (normal behaviour)
- a-group-.*  # this has no explicit start or end, so it will match EXACTLY ^a-group-.*$
- ^a-group    # this has an explicit start, so it will match EVERYTHING that starts with a-group
- a-group$    # this has an explicit end, so it will match EVERYTHING that ends with a-group
```

See `example/composition-regex.yaml` for a complete example.

### Function Response Caching

You can set `cacheTTL` to control the Function response cache time-to-live.
This is useful for tuning reconciliation behavior in large compositions.

```yaml
  - step: sequence-creation
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      cacheTTL: 5m
      rules:
        - sequence:
          - first
          - second
```

### Composite Readiness
Enabling the `resetCompositeReadiness` flag causes the function to set the Composite's `Ready` flag to `False` when at
least one desired resource is deleted from the request. This prevents the Composite resource from entering the `Ready`
state prematurely when there are pending resources that the composite reconciler is unaware of.

```yaml
  - step: sequence-creation
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      resetCompositeReadiness: true
      rules:
        - sequence:
          - first-subresource-.*
          - second-resource
```
## Deletion Sequencing
The same rule sequences can be used to determine the order in which the resources should be deleted.
Deletion Sequencing is enabled by setting the `enableDeletionSequencing` input to `true` and causes the function to create
`Usage` and `ClusterUsage` resources to enforce the proper order of resource deletion.

Deletion Sequencing requires that
[foreground cascading deletion](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#foreground-deletion)
is used when the composite resource is deleted.

The `usageVersion` input attribute controls
whether the function creates Crossplane v1 `Usage.apiextensions.crossplane.io` resources or Crossplane v2
`Usage.protection.crossplane.io` and `ClusterUsage.protection.crossplane.io` resources.

The `replayDeletion` input is mapped to the `Usage`/`ClusterUsage` `replayDeletion` attribute which determines whether 
deletion of a resource is retried after the initial attempt.  This can significantly reduce the amount of time
required to delete all the resources in a composite and defaults to `true` for the deletion
sequencing use case.  This can be disabled by setting `replayDeletion` to `false`.

```yaml
  - step: sequence-creation-and-deletion
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      enableDeletionSequencing: true
      replayDeletion: true
      rules:
        - sequence:
          - first
          - second
          - third
      usageVersion: v1
```
creates two `Usage` resources, one for the `third`->`second` dependency and one for the `second`->`first` dependency.
When the composite is deleted with the option `--cascade=foreground` the `third` resource will be deleted, followed by
the `second` and finally the `first`.

### Regular Expressions

Deletion sequencing creates `Usage`/`ClusterUsage` resources for all dependencies identified by the input sequences, including
those defined by pattern matching.  For example:

```yaml
  - step: sequence-creation-and-deletion
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      enableDeletionSequencing: true
      replayDeletion: true
      rules:
        - sequence:
          - first-subresource-.*
          - second-resource
      usageVersion: v1
```

creates a `Usage` resource for every resource that matches `first-subresource-*`, with `by` set to the `second-resource`.
This ensures that `second-resource` is deleted before any of the `first-resource-*` resources are deleted.

## Delete-Only Mode

By default, sequencing rules enforce ordering for both resource creation and deletion (if `enableDeletionSequencing`).
When `deleteOnly` is set to `true` on a rule, creation sequencing is skipped and resources are not blocked from being created,
even if their predecessors aren't ready (think of it as "rely on eventual consistency" mode).
Deletion ordering is still enforced via `Usage`/`ClusterUsage` resources when `enableDeletionSequencing` is also enabled.

This is useful when you want to rely on eventual consistency for resource creation, but still need guaranteed deletion ordering.

```yaml
  - step: sequence-deletion-only
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      enableDeletionSequencing: true
      rules:
        - sequence:
          - vpc
          - subnet
          - security-group
          deleteOnly: true
```

In the example above, `subnet` and `security-group` will be created immediately without waiting for predecessors.
On deletion, `Usage` resources enforce the reverse order: `security-group` --> `subnet` --> `vpc`.

The `deleteOnly` flag is per-rule, so you can mix creation-sequenced and delete-only rules in the same composition:

```yaml
      rules:
        - sequence:
          - vpc
          - subnet
        - sequence:
          - subnet
          - security-group
          deleteOnly: true
```

## Create-Only Mode

The inverse of `deleteOnly`. When `createOnly` is set to `true` on a rule, creation sequencing is enforced as normal,
but no `Usage`/`ClusterUsage` resources are generated for that rule — even when `enableDeletionSequencing` is enabled.

This is useful when you need strict creation ordering but don't care about deletion ordering for a particular sequence.

```yaml
  - step: sequence-creation-only
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      enableDeletionSequencing: true
      rules:
        - sequence:
          - vpc
          - subnet
          - security-group
          createOnly: true
```

In the example above, `subnet` waits for `vpc` and `security-group` waits for `subnet` before being created.
On deletion, resources are deleted without ordering constraints.

> **Note:** `createOnly` and `deleteOnly` are mutually exclusive on the same rule, setting both options ends up in error.

## Conditional Sequences

Rules can include a `condition` field containing a [CEL](https://github.com/google/cel-spec) (Common Expression Language) expression.
When the condition evaluates to `false`, the entire sequence is skipped and no resources are blocked/removed from the desired state.

This is useful when a sequence involves resources that are conditionally created based on the XR parameters.
Without a condition, a resource that is never created would permanently block all successors in its sequence.

```yaml
  - step: sequence-creation
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      enableDeletionSequencing: true
      rules:
        - sequence:
          - vpc
          - subnet
        - sequence:
          - subnet
          - nat-gateway
          condition: "observed.composite.resource.spec.enableNatGateway == true"
```

When `spec.enableNatGateway` is `false`, the second sequence is skipped entirely.
The `nat-gateway` resource is not blocked, and it does not affect composite readiness.

### CEL Variables

The following variables are available in condition expressions, matching the conventions used by
[function-cel-filter](https://github.com/crossplane-contrib/function-cel-filter):

| Variable | Type | Description |
|----------|------|-------------|
| `observed` | `State` | The observed state of the composite and composed resources |
| `desired` | `State` | The desired state as accumulated by prior pipeline steps |
| `context` | `Struct` | The function pipeline context |

Common access patterns:

```cel
observed.composite.resource.spec.someField == "value"
desired.composite.resource.spec.count > 0
size(observed.resources) > 0
```

### Safety: Observed Resource Protection

When a condition evaluates to `false`, but resources from the sequence already exist (they were created when the condition was previously `true`),
deletion ordering is still enforced.
That is, (`Cluster`)`Usage` resources continue to be generated for observed resources, regardless of the condition.

This prevents resources from being deleted out of order when a condition changes at runtime (e.g. a flaky condition).

### CEL Environment

The CEL environment is lazily initialized, thus there is zero overhead for compositions that do not use conditions.
The environment is created once on first use and reused for subsequent evaluations.

## Installation

The function can be installed into a Crossplane cluster using the following manifest:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-sequencer
spec:
  package: xpkg.crossplane.io/crossplane-contrib/function-sequencer:v0.5.0
```

## Developing this function

You can use `go run` to run your function locally
```sh
go run . --insecure --debug
```

After that, you can use [Crossplane CLI's](https://docs.crossplane.io/latest/cli/) `crossplane render` to test what your function is doing.

To test a sequence:
```sh
# --observed-resources allows you to simulate already created objects
crossplane render example/xr.yaml example/composition.yaml example/functions.yaml --observed-resources example/observed.yaml
```

To test a regex sequence:
```sh
# --observed-resources allows you to simulate already created objects
crossplane render example/xr.yaml example/composition-regex.yaml example/functions.yaml --observed-resources example/observed-regex.yaml
```
