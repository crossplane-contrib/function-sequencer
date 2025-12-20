# Example manifests

You can run your function locally and test it using `crossplane beta render`
with these example manifests.

### Run the function locally

```shell
$ go run . --insecure --debug
```

### Then, in another terminal, call it with these example manifests

```shell
$ crossplane render xr.yaml composition.yaml functions.yaml -r
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: I was run with input "Hello world"!
severity: SEVERITY_NORMAL
step: run-the-template
```

### Regex Pattern Example
```shell
$ crossplane render xr.yaml composition-regex.yaml functions.yaml -r -o observed-regex.yaml
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    message: 'Unready resources: second-object'
    reason: Creating
    status: "False"
    type: Ready
---
apiVersion: nop.crossplane.io/v1alpha1
kind: NopResource
metadata:
  annotations:
    crossplane.io/composition-resource-name: first-subresource-1
  labels:
    crossplane.io/composite: example-xr
  name: first-subresource-1
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    conditionAfter:
    - conditionStatus: "False"
      conditionType: Ready
      time: 5s
    - conditionStatus: "True"
      conditionType: Ready
      time: 10s
    - conditionStatus: "False"
      conditionType: Ready
      time: 30s
    - conditionStatus: "True"
      conditionType: Ready
      time: 90s
---
apiVersion: nop.crossplane.io/v1alpha1
kind: NopResource
metadata:
  annotations:
    crossplane.io/composition-resource-name: first-subresource-2
  labels:
    crossplane.io/composite: example-xr
  name: first-subresource-2
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    conditionAfter:
    - conditionStatus: "False"
      conditionType: Ready
      time: 5s
    - conditionStatus: "True"
      conditionType: Ready
      time: 10s
---
apiVersion: nop.crossplane.io/v1alpha1
kind: NopResource
metadata:
  annotations:
    crossplane.io/composition-resource-name: second-object
  labels:
    crossplane.io/composite: example-xr
  name: second-object
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    conditionAfter:
    - conditionStatus: "False"
      conditionType: Ready
      time: 5s
    - conditionStatus: "True"
      conditionType: Ready
      time: 10s
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: Delaying creation of resource(s) matching "third-resource" because "object$"
  is not fully ready (0 of 1)
severity: SEVERITY_NORMAL
step: sequence-creation
```
### Deletion Sequencing Example
```shell
$ crossplane render xr.yaml composition-with-deletion-sequencing.yaml functions.yaml -r -o observed-deletion-sequencing.yaml
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: Available
    status: "True"
    type: Ready
---
apiVersion: nop.crossplane.io/v1alpha1
kind: NopResource
metadata:
  annotations:
    crossplane.io/composition-resource-name: first-resource
  labels:
    crossplane.io/composite: example-xr
  name: first
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    conditionAfter:
    - conditionStatus: "False"
      conditionType: Ready
      time: 5s
    - conditionStatus: "True"
      conditionType: Ready
      time: 10s
    - conditionStatus: "False"
      conditionType: Ready
      time: 30s
    - conditionStatus: "True"
      conditionType: Ready
      time: 90s
---
apiVersion: nop.crossplane.io/v1alpha1
kind: NopResource
metadata:
  annotations:
    crossplane.io/composition-resource-name: second-resource
  labels:
    crossplane.io/composite: example-xr
  name: second
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    conditionAfter:
    - conditionStatus: "False"
      conditionType: Ready
      time: 5s
    - conditionStatus: "True"
      conditionType: Ready
      time: 10s
---
apiVersion: apiextensions.crossplane.io/v1beta1
kind: Usage
metadata:
  annotations:
    crossplane.io/composition-resource-name: second-resource-first-resource-usage
  labels:
    crossplane.io/composite: example-xr
  name: nopresource-second-nopresource-first-4f1a57-dependency
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  by:
    apiVersion: nop.crossplane.io/v1alpha1
    kind: NopResource
    resourceRef:
      name: second
  of:
    apiVersion: nop.crossplane.io/v1alpha1
    kind: NopResource
    resourceRef:
      name: first
  reason: dependency
  replayDeletion: true
---
apiVersion: nop.crossplane.io/v1alpha1
kind: NopResource
metadata:
  annotations:
    crossplane.io/composition-resource-name: third-resource
  labels:
    crossplane.io/composite: example-xr
  name: third
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    conditionAfter:
    - conditionStatus: "False"
      conditionType: Ready
      time: 5s
    - conditionStatus: "True"
      conditionType: Ready
      time: 10s
---
apiVersion: apiextensions.crossplane.io/v1beta1
kind: Usage
metadata:
  annotations:
    crossplane.io/composition-resource-name: third-resource-first-resource-usage
  labels:
    crossplane.io/composite: example-xr
  name: nopresource-third-nopresource-first-1b4fc5-dependency
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  by:
    apiVersion: nop.crossplane.io/v1alpha1
    kind: NopResource
    resourceRef:
      name: third
  of:
    apiVersion: nop.crossplane.io/v1alpha1
    kind: NopResource
    resourceRef:
      name: first
  reason: dependency
  replayDeletion: true
```
