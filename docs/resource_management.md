# Resource management

This document describes the fundamentals of resource management in permissions-api.

## Resource types

Every resource in permissions-api has a type that defines the actions and relationships that can be scoped to that resource. By default, the only two resource types that exist in permissions-api are _role_ and _tenant_, which define sets of allowed actions and operational context respectively and are necessary for the service to function.

## Resource lifecycle events

permissions-api consumes resource lifecycle events over [NATS][nats]. This section describes the subjects and message formats the service expects.

[nats]: https://nats.io

### Message format

permission-api expects lifecycle event messages in the format described in [`go.infratographer.com/x/pubsubx`][pubsubx], and interprets the message fields as follows:

* `subject_urn`: The URN of the resource the event is about
* `event_type`: The type of lifecyle event for the resouce. Must match one of the defined lifecycle events
* `fields`: Information related to the resource. A field's value will be persisted in permissions-api if the field is of the form `{foo}_urn` and a defined relationship exists on the resource with relation `{foo}`

[pubsubx]: https://github.com/infratographer/x/blob/v0.0.7/pubsubx/message.go

### Subjects

When listening for events, permissions-api subscribes to subjects of the form `{namespace}.{resource_type}.{lifecycle_event}`, where `namespace` is the configured namespace for resource types, `resource_type` is any resource type not directly managed by permissions-api, and `lifecycle_event` is one of the following defined lifecycle events.

### Lifecycle events

The following lifecycle events are recognized by permissions-api and will result in the described effects.

#### `create`

A `create` event signals that a given resource has been created. All relationships described in the given message will be created. It is an error to send multiple `create` events for a single resource.

#### `update`

An `update` event signals that a given resource has been updated. All relationships described in the given event message will be created if they do not already exist, and all other relationships not defined in the message will be deleted. Sending multiple `update` events for the same resource is permitted.

#### `delete`

A `delete` event signals that a given resource has been deleted. All relationships will be deleted where the resource in the relationship (as opposed to the subject) matches the provided resource in the message. Note that this means resource deletion in permissions-api will not result in cascading deletes of relationships. It is the responsibility of all services that manage resources to send explicit `delete` events for every resource they manage.

### Example

As an example, consider the following lifecycle event for a resource of type `loadbalancer`:

```json
{
  "subject_urn": "urn:infratographer:loadbalancer:0e919c70-6d04-4050-a474-073ab8b58ffe",
  "event_type": "create",
  "additional_subjects": [
    "urn:infratographer:tenant:42f0e8f2-4b81-4e5a-86f2-62d78ed35dca",
    "urn:infratographer:loadbalancerport:db25eabd-30eb-4654-9bb6-a22c140eac97",
    "urn:infratographer:loadbalancerassignment:44cadf84-c626-4428-8910-3a699a78b898"
  ],
  "actor_urn": "urn:infratographer:user:35464f0b-a7b4-47db-b446-01e61987db6c",
  "source": "loadbalancer-api",
  "timestamp": "2023-05-06T17:30:00Z",
  "fields": {
    "tenant_urn": "urn:infratographer:tenant:42f0e8f2-4b81-4e5a-86f2-62d78ed35dca"
  },
  "additional_data": {}
}
```

permissions-api will interpret this as the creation of a `loadbalancer` resource, and attempt to create a relationship of the form:

```
infratographer/loadbalancer:0e919c70-6d04-4050-a474-073ab8b58ffe tenant infratographer/tenant:42f0e8f2-4b81-4e5a-86f2-62d78ed35dca
```
