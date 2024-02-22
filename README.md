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
      apiVersion: template.fn.crossplane.io/v1beta1
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

Currently it could be installed as follows:

```
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-sequencer
spec:
  package: xpkg.upbound.io/hasan/function-sequencer:v0.0.3
```


