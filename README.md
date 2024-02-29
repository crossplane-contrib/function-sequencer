# function-sequencer

Function Sequencer is a Crossplane function that enables Composition authors to define sequencing rules delaying the
creation of resources until other resources are ready.

For example, the pipeline step below, will ensure that `second-resource` and `third-resource` not to be created until
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

## Installation

It can be installed as follows from the Upbound marketplace: https://marketplace.upbound.io/functions/crossplane-contrib/function-sequencer
