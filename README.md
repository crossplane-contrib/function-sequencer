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

See `example/composition-regex.yaml` for a complete example.

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
