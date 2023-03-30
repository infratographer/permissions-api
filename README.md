# Permissions API

Hello, and welcome to the Permissions API! This API is designed to allow you to manage permissions for services in the Infratographer ecosystem. The intent is for this project to contain both the API to manage permissions, as well as the policy engine that will be used to enforce permissions.

## Concepts

Before we get into the details of the API, let's talk about some of the concepts that are important to understanding how the API works.

### Subject

A subject is a user, group, or service that is requesting access to a resource.

### Resource

A resource is a tenant or object that is being accessed.

In an initial implementation, the resource will be a tenant. In the future, we will add support for objects.

### Action

An action is a verb that describes what the subject is trying to do to the resource. For example, "read", "write", "delete", "create", etc.

Given that Permissions API is designed to be used by multiple services, the actions are currently defined by the service that is using the API. e.g. a Load Balancer service may define the actions "loadbalancers_get", "loadbalancers_create", "loadbalancers_delete", etc.

### Role

A role is a collection of actions that are allowed to be performed on a resource.

### Role Assignment

A role assignment is a mapping of a subject to a role. This is how a subject is granted access to a resource.

# Components

The Permissions API is made up of two components:

* Management API
* Policy Engine
