# kngw
Knative gateway

## How to build?
```bash
pack build --builder=gcr.io/buildpacks/builder:v1 --publish wei840222/kngw:13
```

## How to deploy?
```bash
kn ksvc apply --namespace=knative-serving --image=wei840222/kngw:13 --scale-min=1 --annotation=prometheus.io/scrape=true --annotation=prometheus.io/port=2222 --annotation=instrumentation.opentelemetry.io/inject-sdk=true gateway
```
