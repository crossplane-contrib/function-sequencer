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

## Installation

It can be installed as follows from the Upbound marketplace: https://marketplace.upbound.io/functions/crossplane-contrib/function-sequencer

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
