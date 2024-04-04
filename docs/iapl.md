* Infratographer authorization policy language (rev 2)

** Background

During self-registration of resources in Infratographer, it is expected that service authors will want to provide their own authorization policies for resources, as well as the set of relationships those resources have to other resources. This document covers an authorization policy language for resources, referred from here onwards as the Infratographer authorization policy language (or just authorization policy language).

** Design

Under this design, all resources are defined in terms of their relationships to other resources, possible actions on those resources, and conditions that must be met for those actions to be allowed.

*** Policy schema

This section covers the definitions and grammar of a policy in the Infratographer authorization policy language.

**** ~Policy~

A ~Policy~ describes the complete authorization policy for an Infratographer deployment. It is a YAML stream of ~PolicyDocument~ objects, which can be provided in one or more files. During policy validation, all documents in the authorization policy are merged into a single ~PolicyDocument~. The merge occurs only at the level of the highest keys - ~ResourceTypes~, ~Unions~, ~Actions~, and ~ActionBindings~. If duplicate objects (named objects with the same name, or unnamed objects such as ~ActionBindings~ with the same key attributes) are detected in the final merged document, an error is thrown during the validation phase. Since there is no nested object merging, the order of objects in the YAML (or the order in which policy YAML files are provided) is inconsequential. See the policy validation algorithm below.

**** ~PolicyDocument~

A ~PolicyDocument~ describes some part of an authorization policy for an Infratographer deployment in terms of a set of resources (likely provided by a single service). It is a YAML document that contains a single mapping, which itself contains the following keys:

| Key              | Type              | Description                                                                                  |
|------------------+-------------------+----------------------------------------------------------------------------------------------|
| ~resourceTypes~  | ~[]ResourceType~  | A list of ~ResourceType~ objects that define the resource types in the authorization policy. |
| ~unions~         | ~[]Union~         | A list of ~Union~ objects that give a common name to multiple types.                         |
| ~actions~        | ~[]Action~        | A list of ~Action~ objects that define the available actions in the authorization policy.    |
| ~actionBindings~ | ~[]ActionBinding~ | A list of ~ActionBinding~ objects binding resource types to actions.                         |

**** ~ResourceType~

A ~ResourceType~ describes the authorization policy for a single resource in terms of its corresponding type in Infratographer, relationships to other resources, and actions that can be performed on a resource of the given type. It is a YAML mapping that contains the following keys:

| Key             | Type             | Description                                                                         |
|-----------------+------------------+-------------------------------------------------------------------------------------|
| ~name~          | ~string~         | The name of the type. Must be all alphanumeric characters.                          |
| ~idPrefix~      | ~string~         | The Infratographer ID prefix for a resource of this type.                           |
| ~relationships~ | ~[]Relationship~ | A list of ~Relationship~ objects describing this type's relation to other types.    |

**** ~Union~

A ~Union~ describes a named sum of any number of concrete resource types. It is a YAML mapping that contains the following keys:

| Key                 | Type     | Description                                                                                            |
|---------------------+----------+--------------------------------------------------------------------------------------------------------|
| ~name~              | ~string~ | The name of the union. Must be all alphanumeric characters.                                            |
| ~resourceTypeNames~ | ~string~ | The underlying resource types that this alias can refer to. Must include only concrete resource types. |

**** ~Relationship~

A ~Relationship~ describes a named relation between a resource of a given type and a resource of another type. It is a YAML mapping that contains the following keys:

| Key               | Type       | Description                                                                                            |
|-------------------+------------+--------------------------------------------------------------------------------------------------------|
| ~relation~        | ~string~   | The name of the relationship. Must be all alphabetical.                                                |
| ~targetTypeNames~ | ~[]string~ | The types of resources on the other side of the relationship. Must be defined resource type or unions. |

Specifying a ~targetTypeName~ value of ~[foo]~ where ~foo~ is a union over types ~bar~ and ~baz~ is equivalent to specifying a value of ~[bar, baz]~.

**** ~Action~

An ~Action~ describes an action that can be taken on a resource. Actions are predicated on conditions, and are allowed if any condition is satisfied. It is a YAML mapping that contains the following keys:

| Key          | Type          | Description                                                           |
|--------------+---------------+-----------------------------------------------------------------------|
| ~name~       | ~string~      | The name of the action. Must be valid using the regex ~[a-z][a-z_]+~. |

**** ~ActionBinding~

An ~ActionBinding~ describes a binding of an action to a resource type, where both the action and resource type are defined in the authorization policy document. It is a YAML mapping that contains the following keys:

| Key          | Type          | Description                                                                                  |
|--------------+---------------+----------------------------------------------------------------------------------------------|
| ~actionName~ | ~string~      | The name of the action to bind to the resource type. Must be defined in the policy.          |
| ~typeName~   | ~string~      | The name of the resource type or union to bind to the action. Must be defined in the policy. |
| ~conditions~ | ~[]Condition~ | The conditions under which the action will be allowed.                                       |

Specifying a ~typeName~ value of ~foo~ where ~foo~ is a union over types ~bar~ and ~baz~ is equivalent to defining the same ~ActionBinding~ on types ~bar~ and ~baz~.

**** ~Condition~

A ~Condition~ describes a necessary condition for allowing an action to proceed. It is a YAML mapping that contains the following keys, all exclusive of each other:

| Key                  | Type                          | Description                                                                                          |
|----------------------+-------------------------------+------------------------------------------------------------------------------------------------------|
| ~roleBinding~        | ~ConditionRoleBinding~        | Denotes that this action can be allowed via a role binding.                                          |
| ~relationshipAction~ | ~ConditionRelationshipAction~ | Denotes that this action can be allowed if an action is allowed on a relationship's target resource. |

**** ~ConditionRoleBinding~

A ~ConditionRoleBinding~ describes a condition where a role binding will allow a given action. It is an empty YAML mapping (generally a literal ~{}~).

**** ~ConditionRelationshipAction~

A ~ConditionRelationshipAction~ describes a condition that will allow an action if another action is allowed on a target resource of a relationship. It is a YAML mapping that contains the following keys:

| Key            | Type     | Description                                                                                              |
|----------------+----------+----------------------------------------------------------------------------------------------------------|
| ~relation~     | ~string~ | A relation. Must refer to a defined relationship for a resource of the enclosing resource type.  |
| ~actionName~   | ~string~ | An action name. Must refer to a defined action for a resource of the relationship's target type. |

*** Example

The following policy document describes a load balancer resource, tenant resource, organization resource, project resource, and aliases and actions. In plain language, the policy reads something like so:

- Load balancers belong to tenants or projects
- Load balancers can be viewed if there is a direct role binding or the owner allows viewing load balancers
- Load balancers can be created and viewed within tenants if there is a direct role binding or the parent allows it
- Load balancers can be created and viewed within projects if there is a direct role binding or the organization allows it

#+BEGIN_SRC yaml
  # Provided by tenant-api
  resourceTypes:
    - name: tenant
      idPrefix: idntten
      relationships:
        - relation: parent
          targetTypeNames:
            - tenant
  ---
  # Provided by enterprise-api
  resourceTypes:
    - name: project
      idPrefix: entrprj
      relationships:
        - relation: parent
          targetTypeNames:
            - organization
    - name: organization
      idPrefix: entrorg
      relationships:
        - relation: parent
          targetTypeNames:
            - tenant
  ---
  # Provided by load-balancer-api
  resourceTypes:
    - name: loadbalancer
      idPrefix: loadbal
      relationships:
        - relation: owner
          targetTypeNames:
            - resourceowner
  actions:
    - name: loadbalancer_get
    - name: loadbalancer_create
  actionBindings:
    - actionName: loadbalancer_get
      typeName: loadbalancer
      conditions:
        - roleBinding: {}
        - relationshipAction:
            relation: owner
            actionName: loadbalancer_get
    - actionName: loadbalancer_get
      typeName: resourceowner
      conditions:
        - roleBinding: {}
        - relationshipAction:
            relation: parent
            actionName: loadbalancer_get
    - actionName: loadbalancer_create
      typeName: loadbalancer
      conditions:
        - roleBinding: {}
        - relationshipAction:
            relation: owner
            actionName: loadbalancer_create
    - actionName: loadbalancer_create
      typeName: resourceowner
      conditions:
        - roleBinding: {}
        - relationshipAction:
            relation: parent
            actionName: loadbalancer_create
  ---
  # Provided by resource-owner-config
  unions:
    - name: resourceowner
      resourceTypeNames:
        - tenant
        - project
        - organization
#+END_SRC

*** Policy validation algorithm

Policies can be validated according to the following algorithm (in Python-like pseudocode):

#+BEGIN_SRC
  RT = {rt.name: rt for rt in resourceTypes}
  UN = {un.name: un for un in unions}
  AC = {ac.name: ac for act in actions}

  # expansion phase

  BN = []
  BNKeys = []
  for bn in actionBindings:
    if bn.typeName in UN:
      for typeName in UN[bn.typeName].targetTypeNames:
        BN += [
          ActionBinding(
            typeName: typeName,
            actionName: bn.actionName,
            conditions: bn.conditions,
          ),
        ]
        if (typeName, bn.actionName) in BNKeys:
          fail()
        else:
          BNKeys += (typeName, bn.actionName)
    else:
      BN += bn
      fail()

  for rt in RT:
    rels = []
    for rel in rt.relationships:
      typeNames = []
      for typeName in rel.targetTypeNames:
        if typeName in UN:
          typeNames += UN[typeName].resourceTypeNames
        else:
          typeNames += [typeName]
      rel.typeNames = typeNames
      rels += [rel]

    rt.relationships = rels

  RB = {}
  for bn in BN:
    RB.setdefault(bn.typeName, set()).add(bn.actionName)

  # validation phase

  for un in UN:
    for name in un.resourceTypeNames:
      assert name in UN

  for rt in RT:
    for rel in rt.relationships:
      for tn in rel.targetTypeNames:
        assert tn in RT

  for bn in BN:
    assert bn.actionName in AC
    assert bn.typeName in RT

    rt = RT[bn.resourceTypeName]

    for c in bn.conditions:
      assert (c.roleBinding or c.relationshipAction) and not (c.roleBinding and c.relationshipAction)

      if c.relationshipAction:
        rel = find(rt.relationships, lambda x: c.relation == x.relation)
        assert rel

        for tn in rel.targetTypeNames:
          assert bn.actionName in RB[tn]
#+END_SRC

*** 

*** Mapping policies to SpiceDB

SpiceDB is the reference implementation for authorization in Infratographer, and policies can be mapped to SpiceDB schemas. The mapping from policy to SpiceDB schema is as follows:

- Every ~ResourceType~ has a corresponding SpiceDB definition
- Every ~Relationship~ has a corresponding SpiceDB relation in the resource type's corresponding definition
- Every ~ActionBinding~ has both a corresponding SpiceDB relation and permission in SpiceDB definition for the the action binding's resource type
- Every ~Condition~ has a corresponding clause in its action binding's permission
- Every reference to a type alias maps to a list of all of that alias's concrete underlying types

Given these mappings, the example policy defined above might map to a partial SpiceDB schema like so (role is omitted for brevity):

#+BEGIN_SRC
  definition infratographer/tenant {
    relation parent: infratographer/tenant

    relation loadbalancer_create_role: infratographer/role#subject
    relation loadbalancer_get_role: infratographer/role#subject

    permission loadbalancer_create = loadbalancer_create_role + parent->loadbalancer_create
    permission loadbalancer_get = loadbalancer_get_role + parent->loadbalancer_get
  }
  
  definition infratographer/project {
    relation organization: infratographer/organization

    relation loadbalancer_create_role: infratographer/role#subject
    relation loadbalancer_get_role: infratographer/role#subject

    permission loadbalancer_create = loadbalancer_create_role + parent->loadbalancer_create
    permission loadbalancer_get = loadbalancer_get_role + parent->loadbalancer_get
  }

  definition infratographer/organization {
    relation parent: infratographer/tenant

    relation loadbalancer_create_role: infratographer/role#subject
    relation loadbalancer_get_role: infratographer/role#subject

    permission loadbalancer_create = loadbalancer_create_role + parent->loadbalancer_create
    permission loadbalancer_get = loadbalancer_get_role + parent->loadbalancer_get
  }

  definition infratographer/loadbalancer {
    relation owner: infratographer/tenant | infratographer/project

    relation loadbalancer_get_role: infratographer/role#subject

    permission loadbalancer_get = loadbalancer_get_role + owner->loadbalancer_get
  }
#+END_SRC
