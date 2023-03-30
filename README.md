# permissions-api - Permissions management service

Hello, and welcome to permissions-api! permissions-api is a service designed for checking and managing permissions on resources in the Infratographer ecosystem.

permissions-api is made up of two components:

* A policy decision endpoint for making permissions checks
* A management API for resources necessary to make decisions

For information around the concepts underlying permissions-api, see the [Concepts](#concepts) section of this README.

## Usage

permissions-api is a Go service. To build it, you can use `make build` to build a Go binary.

### Creating relationships

Resources are defined in terms of their relationships to other resources using the `/relationships` API endpoint. Using curl, one can create a relationship `tenant` between two tenants like so:

```
$ curl --oauth2-bearer "$AUTH_TOKEN" \
    -d '{"relationships": [{"relation": "tenant", "subject_urn": "urn:infratographer:tenant:075b8c8c-1214-49ac-a7ed-ec102f165568"}]}' \
    http://localhost:7602/api/v1/resources/urn:infratographer:tenant:3fc4e4e0-6030-4e36-83d6-09ae2d58fee8/relationships
```

### Creating roles

Roles are created using the `/roles` API endpoint. For example, the following curl command creates a role scoped to a tenant that allows the `loadbalancer_create` action:

```
$ curl --oauth2-bearer "$AUTH_TOKEN" \
    -d '{"actions": ["loadbalancer_create"]}' \
    http://localhost:7602/api/v1/resources/urn:infratographer:tenant:3fc4e4e0-6030-4e36-83d6-09ae2d58fee8/roles
```

### Assigning roles to subjects

Roles are assigned to subjects using the `/assignments` API endpoint. The curl command below will assign the subject with the given URN to the given role:

```
$ curl --oauth2-bearer "$AUTH_TOKEN" \
    -d '{"subject_urn": "urn:infratographer:user:e0a97b60-af68-4376-828c-78c6c2ab04a9"}' \
    http://localhost:7602/api/v1/roles/7a5ccbfb-6838-478e-9d61-cffd38ceb5a3/assignments
```

### Checking permissions

The `/has` API endpoint is used to check whether the authenticated subject in the given bearer token has permission to perform the requested action on the given resource. The following example checks to see whether a subject can perform the `loadbalancer_create` operation on a tenant:

```
$ curl --oauth2-bearer "$AUTH_TOKEN" \
    http://localhost:7602/api/v1/has/loadbalancer_create/on/urn:infratographer:tenant:3fc4e4e0-6030-4e36-83d6-09ae2d58fee8
```

## Concepts

permissions-api is designed to answer the following question: Does the given _subject_ have permission to perform the requested _action_ on the requested _resource_?

Permission is granted by the assignment of roles to subjects.

The concepts necessary to accomplish this are described in this section.

### Resource

A resource is any uniquely identifiable thing in the Infratographer ecosystem. Resources have types and are identified using URNs in permissions-api. For example, the URN `urn:infratographer:loadbalancer:a69e79ce-ac05-4a3e-b9b3-f371255e8e99` corresponds to a resource of type `loadbalancer` and ID `a69e79ce-ac05-4a3e-b9b3-f371255e8e99`.

### Subject

A subject is any resource that can be granted permission to perform some action. A subject may be a user, an OAuth client, a server instance, or any other resource.

### Relationship

A relationship is a named link between two resources. Resources in permissions-api are defined entirely in terms of their relationships.

### Action

An action is a relationship that describes something that can be done to a resource (e.g., "update"). Actions are scoped to and named based on resources. For example, an action called `update` scoped to the `loadbalancer` resource is referred to as `loadbalancer_update`.

### Role

A role is a collection of actions that are allowed to be performed on a resource.

### Role assignment

A role assignment is a relationship that binds a subject to a role. This is how a subject is granted access to a resource.

## Development

identity-api includes a [dev container][dev-container] for facilitating service development. Using the dev container is not required, but provides a consistent environment for all contributors as well as a few perks like:

* [gopls][gopls] integration out of the box
* Host SSH auth socket mount
* Git support
* Auxiliary services (SpiceDB, CRDB, etc)

To get started, you can use either [VS Code][vs-code] or the official [CLI][cli].

[dev-container]: https://containers.dev/
[gopls]: https://pkg.go.dev/golang.org/x/tools/gopls
[vs-code]: https://code.visualstudio.com/docs/devcontainers/containers
[cli]: https://github.com/devcontainers/cli

### Manually setting up SSH agent forwarding

The provided dev container listens for SSH connections on port 2222 and bind mounts `~/.ssh/authorized_keys` from the host to facilitate SSH. In order to perform Git operations (i.e., committing code in the container), you will need to enable SSH agent forwarding from your machine to the dev container. While VS Code handles this automatically, for other editors you will need to set this up manually.

To do so, update your `~/.ssh/config` to support agent forwarding. The following config snippet assumes the existence of a remote host used for development and uses it to make permissions-api reachable at the host `permissions-api-devcontainer`:

```
Host permissions-api-devcontainer
  ProxyJump YOUR_HOST_HERE
  Port 2224
  User vscode
  ForwardAgent yes

Host YOUR_HOST_HERE
  User YOUR_USER_HERE
  ForwardAgent yes
```

See the man page for `ssh_config` for more information on what these options do.
