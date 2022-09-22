# kngw
Knative function gateway

## How to build?
```bash
pack build --builder=gcr.io/buildpacks/builder:v1 --publish wei840222/kngw:1
```

## How to deploy?
```bash
kn ksvc apply --namespace=knative-serving --image=wei840222/kngw:1 --scale-min=1 gateway
```