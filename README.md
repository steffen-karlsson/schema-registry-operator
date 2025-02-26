# Schema Registry Operator

A Kubernetes Operator for deploying and managing the Confluent Schema Registry and Schema objects.

### Features
- Declarative `Schema Registry` management via CRDs
- Declarative `Schema` management via CRDs

### Examples

**Schema Registry**
```yaml
apiVersion: client.sroperator.io/v1alpha1
kind: SchemaRegistry
metadata:
  name: schemaregistry-sample
  namespace: schema-registry-operator-system
spec:
  image:
    tag: 6.1.0
    repository: docker.io/confluentinc/cp-schema-registry
    pullPolicy: IfNotPresent
  replicas: 1
  compatibilityLevel: BACKWARD
  resources:
    requests:
      memory: 1Gi
      cpu: 2
    limits:
      memory: 2Gi
      cpu: 2
  ingress:
    enabled: true
    host: my-schema-registry.com
  metrics:
    enabled: true
    port: 9404
  debug: true
  kafkaConfig:
    bootstrapServers:
      - test-cluster-kafka-bootstrap:9094
    authentication:
      saslJaasConfig:
        valueFrom:
          secretKeyRef:
            name: my-cluster-cluster-admin
            key: sasl.jaas.config
```

**Schema**
```yaml
apiVersion: client.sroperator.io/v1alpha1
kind: Schema
metadata:
  labels:
    client.sroperator.io/instance: schemaregistry-sample
  name: schema-sample
  namespace: schema-registry-operator-system
spec:
  target: VALUE
  type: AVRO
  compatibilityLevel: BACKWARD
  content: |
    {
        "type": "record",
        "name": "test",
        "fields": [
            {
                "type": "string",
                "name": "field1"
            },
            {
                "type": "int",
                "name": "field2"
            }
        ]
    }
```

More examples can be found [here](./config/samples/client_v1alpha1_schema.yaml)

## Development
### Prerequisites
- kind cluster
- kubectl
- go

## Installation
```sh
kind create cluster
make redeploy
```

## Cleaning
```sh
kind delete cluster
```

## License
MIT. See [LICENSE](./LICENSE).

