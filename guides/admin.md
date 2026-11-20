# Admin API

`AdminClient` uses the Pinecone Admin API to manage projects, organizations, API keys, role bindings, service accounts,
invites, and users. Each has its own sub-client: `Project`, `Organization`, `APIKey`, `RoleBinding`, `ServiceAccount`,
`Invite`, and `User`.

> [!NOTE]
> Service accounts, which the Admin API requires, are in public preview and available only on Enterprise plans.

## Initialize an AdminClient

`AdminClient` authenticates with a
[service account](https://docs.pinecone.io/guides/organizations/manage-service-accounts). Organization owners can
create one in the Pinecone console, at
[**Settings > Access > Service accounts**](https://app.pinecone.io/organizations/-/settings/access/service-accounts).

### Authenticate with a client ID and secret

When you create a service account, you get a client ID and secret. Pass them in
`NewAdminClientParams`, or set the `PINECONE_CLIENT_ID` and `PINECONE_CLIENT_SECRET` environment variables.
`NewAdminClient` handles the authentication handshake and returns an authenticated `AdminClient`. The client refreshes
its access token before it expires, so a long-lived `AdminClient` keeps working. To control the handshake's context,
use `NewAdminClientWithContext`.

If the token exchange fails, `NewAdminClient` returns a `*PineconeError`. Its `Message` starts with
`"failed to get auth token: "` followed by the token endpoint's description, and its `ErrorCode` is the OAuth error
code, such as `"access_denied"`. A failed refresh later on returns the same error from the call that needed the token.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/pinecone-io/go-pinecone/v8/pinecone"
)

func main() {
	ctx := context.Background()

	adminClient, err := pinecone.NewAdminClient(pinecone.NewAdminClientParams{
		ClientId:     os.Getenv("PINECONE_CLIENT_ID"),
		ClientSecret: os.Getenv("PINECONE_CLIENT_SECRET"),
	})
	if err != nil {
		log.Fatalf("Failed to create AdminClient: %v", err)
	}

	projects, err := adminClient.Project.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list projects: %v", err)
	}
	fmt.Printf("You have %d project(s)\n", len(projects))
}
```

### Authenticate with an access token

You can instead pass an existing access token as `AccessToken` (or set `PINECONE_ACCESS_TOKEN`). It takes precedence
over a client ID and secret, and is used as-is: it isn't refreshed, so requests fail once it expires.

The examples below assume an `AdminClient` named `adminClient` and a `context.Context` named `ctx`.

## Projects

```go
project, err := adminClient.Project.Create(ctx, &pinecone.CreateProjectParams{
	Name: "example-project",
})
if err != nil {
	log.Fatalf("Failed to create project: %v", err)
}

projects, err := adminClient.Project.List(ctx)
if err != nil {
	log.Fatalf("Failed to list projects: %v", err)
}
fmt.Printf("%d projects\n", len(projects))

project, err = adminClient.Project.Describe(ctx, project.Id)
if err != nil {
	log.Fatalf("Failed to describe project: %v", err)
}

newName := "renamed-project"
project, err = adminClient.Project.Update(ctx, project.Id, &pinecone.UpdateProjectParams{
	Name: &newName,
})
if err != nil {
	log.Fatalf("Failed to update project: %v", err)
}
fmt.Printf("Renamed project to %s\n", project.Name)

err = adminClient.Project.Delete(ctx, project.Id)
if err != nil {
	log.Fatalf("Failed to delete project: %v", err)
}
```

You can't delete a project while it has indexes, collections, assistants, or backups. Its API keys are deleted with it.

`CreateProjectParams` and `UpdateProjectParams` also accept these fields:

- `ForceEncryptionWithCmek`: Forces encryption with customer-managed encryption keys (CMEK). Once enabled, it can't be
  disabled.
- `MaxPods`: The maximum number of pods the project can use. The default, 0, allows serverless indexes only.

## Organizations

```go
orgs, err := adminClient.Organization.List(ctx)
if err != nil {
	log.Fatalf("Failed to list organizations: %v", err)
}

org, err := adminClient.Organization.Describe(ctx, orgs[0].Id)
if err != nil {
	log.Fatalf("Failed to describe organization: %v", err)
}

newName := "renamed-org"
org, err = adminClient.Organization.Update(ctx, org.Id, &pinecone.UpdateOrganizationParams{
	Name: &newName,
})
if err != nil {
	log.Fatalf("Failed to update organization: %v", err)
}
fmt.Printf("Renamed organization to %s\n", org.Name)

err = adminClient.Organization.Delete(ctx, org.Id)
if err != nil {
	log.Fatalf("Failed to delete organization: %v", err)
}
```

`Update` can change only the organization's name. You can delete an organization only when it's on the Free plan, its
payment status is active, and it has no projects.

## API keys

API keys belong to a project. `Create` returns an `APIKeyWithSecret`, whose `Value` is the key to authenticate with.
`Value` is returned only when the key is created.

```go
roles := []string{"ProjectEditor"}

apiKey, err := adminClient.APIKey.Create(ctx, project.Id, &pinecone.CreateAPIKeyParams{
	Name:  "example-api-key",
	Roles: &roles,
})
if err != nil {
	log.Fatalf("Failed to create API key: %v", err)
}
fmt.Printf("Created API key %s\n", apiKey.Key.Id)

keys, err := adminClient.APIKey.List(ctx, project.Id)
if err != nil {
	log.Fatalf("Failed to list API keys: %v", err)
}
fmt.Printf("%d API keys\n", len(keys))

key, err := adminClient.APIKey.Describe(ctx, apiKey.Key.Id)
if err != nil {
	log.Fatalf("Failed to describe API key: %v", err)
}

newRoles := []string{"DataPlaneViewer"}
key, err = adminClient.APIKey.Update(ctx, apiKey.Key.Id, &pinecone.UpdateAPIKeyParams{
	Roles: &newRoles,
})
if err != nil {
	log.Fatalf("Failed to update API key: %v", err)
}
fmt.Printf("API key %s now has roles %v\n", key.Id, key.Roles)

err = adminClient.APIKey.Delete(ctx, apiKey.Key.Id)
if err != nil {
	log.Fatalf("Failed to delete API key: %v", err)
}
```

API key roles are `"ProjectEditor"` (the default), `"ProjectViewer"`, `"ControlPlaneEditor"`, `"ControlPlaneViewer"`,
`"DataPlaneEditor"`, and `"DataPlaneViewer"`. Updating `Roles` replaces the key's existing roles.

## Service accounts

`Create` returns a `ServiceAccountWithSecret`. Its `ClientSecret` is returned only once, when the service account is
created or its secret is rotated, so store it securely and never log it.

```go
projectId := project.Id

sa, err := adminClient.ServiceAccount.Create(ctx, &pinecone.CreateServiceAccountParams{
	Name: "example-service-account",
	RoleBindings: []pinecone.RoleBindingInput{
		{ResourceType: pinecone.ResourceTypeProject, ResourceId: &projectId, Role: "ProjectEditor"},
	},
})
if err != nil {
	log.Fatalf("Failed to create service account: %v", err)
}
fmt.Printf("Client ID: %s\n", sa.ServiceAccount.ClientId)

accounts, err := adminClient.ServiceAccount.List(ctx, &pinecone.ListServiceAccountsParams{})
if err != nil {
	log.Fatalf("Failed to list service accounts: %v", err)
}
fmt.Printf("%d service accounts\n", len(accounts.Data))

account, err := adminClient.ServiceAccount.Describe(ctx, sa.ServiceAccount.Id)
if err != nil {
	log.Fatalf("Failed to describe service account: %v", err)
}

newName := "renamed-service-account"
account, err = adminClient.ServiceAccount.Update(ctx, sa.ServiceAccount.Id, &pinecone.UpdateServiceAccountParams{
	Name: &newName,
})
if err != nil {
	log.Fatalf("Failed to update service account: %v", err)
}
fmt.Printf("Renamed service account to %s\n", account.Name)

rotated, err := adminClient.ServiceAccount.RotateSecret(ctx, sa.ServiceAccount.Id)
if err != nil {
	log.Fatalf("Failed to rotate service account secret: %v", err)
}
// Store rotated.ClientSecret securely, and never log it.
fmt.Printf("Rotated the secret of client %s\n", rotated.ServiceAccount.ClientId)

err = adminClient.ServiceAccount.Delete(ctx, sa.ServiceAccount.Id)
if err != nil {
	log.Fatalf("Failed to delete service account: %v", err)
}
```

`RotateSecret` returns a new client secret, and the previous secret stops working. Access tokens already issued with
the previous secret stay valid until they expire.

## Role bindings

A role binding grants a role to a principal (a user, service account, API key, or invite) at organization or project
scope. For project scope, set `ResourceId` to the project ID. For organization scope, leave it out.

```go
binding, err := adminClient.RoleBinding.Create(ctx, &pinecone.CreateRoleBindingParams{
	PrincipalId:   sa.ServiceAccount.Id,
	PrincipalType: pinecone.PrincipalTypeServiceAccount,
	ResourceType:  pinecone.ResourceTypeProject,
	ResourceId:    &projectId,
	Role:          "ProjectViewer",
})
if err != nil {
	log.Fatalf("Failed to create role binding: %v", err)
}

principalType := pinecone.PrincipalTypeServiceAccount
bindings, err := adminClient.RoleBinding.List(ctx, &pinecone.ListRoleBindingsParams{
	PrincipalType: &principalType,
	PrincipalId:   &sa.ServiceAccount.Id,
})
if err != nil {
	log.Fatalf("Failed to list role bindings: %v", err)
}
fmt.Printf("%d role bindings\n", len(bindings.Data))

binding, err = adminClient.RoleBinding.Describe(ctx, binding.Id)
if err != nil {
	log.Fatalf("Failed to describe role binding: %v", err)
}

err = adminClient.RoleBinding.Delete(ctx, binding.Id)
if err != nil {
	log.Fatalf("Failed to delete role binding: %v", err)
}
```

Organization-scoped roles are `"OrgOwner"`, `"OrgManager"`, `"OrgBillingAdmin"`, and `"OrgMember"`. Project-scoped
roles are `"ProjectOwner"`, `"ProjectManager"`, `"ProjectMember"`, `"ProjectEditor"`, `"ProjectViewer"`,
`"ControlPlaneEditor"`, `"ControlPlaneViewer"`, `"DataPlaneEditor"`, and `"DataPlaneViewer"`.

## Invites

An invite must include at least one organization-scoped role binding that grants membership, and can add
project-scoped bindings.

```go
invite, err := adminClient.Invite.Create(ctx, &pinecone.CreateInviteParams{
	Email: "teammate@example.com",
	RoleBindings: []pinecone.RoleBindingInput{
		{ResourceType: pinecone.ResourceTypeOrganization, Role: "OrgMember"},
		{ResourceType: pinecone.ResourceTypeProject, ResourceId: &projectId, Role: "ProjectEditor"},
	},
})
if err != nil {
	log.Fatalf("Failed to create invite: %v", err)
}

invites, err := adminClient.Invite.List(ctx, &pinecone.ListInvitesParams{})
if err != nil {
	log.Fatalf("Failed to list invites: %v", err)
}
fmt.Printf("%d pending or expired invites\n", len(invites.Data))

invite, err = adminClient.Invite.Describe(ctx, invite.Id)
if err != nil {
	log.Fatalf("Failed to describe invite: %v", err)
}

invite, err = adminClient.Invite.Resend(ctx, invite.Id)
if err != nil {
	log.Fatalf("Failed to resend invite: %v", err)
}
fmt.Printf("Invite to %s expires at %v\n", invite.Email, invite.ExpiresAt)

err = adminClient.Invite.Delete(ctx, invite.Id)
if err != nil {
	log.Fatalf("Failed to delete invite: %v", err)
}
```

`List` returns only pending and expired invites. `Resend` sets the invite back to pending and extends its expiration to
7 days from now. To cancel a pending invite, delete it.

## Users

```go
email := "teammate@example.com"

users, err := adminClient.User.List(ctx, &pinecone.ListUsersParams{Email: &email})
if err != nil {
	log.Fatalf("Failed to list users: %v", err)
}

user, err := adminClient.User.Describe(ctx, users.Data[0].Id)

err = adminClient.User.Delete(ctx, user.Id)
```

The `Email` filter is case-insensitive. Use a user's `Id` as the `PrincipalId` when you create role bindings for them.
When the organization has an SSO connection, a user's `Accounts` lists the accounts they can log in with: one with
`Type` `"sso"` for each account on that connection, with its SAML `NameId`, and one with `Type` `"personal"` if they
also have a personal account. `Accounts` is nil when the organization has no SSO connection.
`Delete` removes the user from the organization and revokes their role bindings. It doesn't delete their Pinecone
account.

## Pagination

`RoleBinding.List`, `ServiceAccount.List`, `Invite.List`, and `User.List` return a page of results, `Limit` (1–100,
default 100) per page. When `Pagination` is non-nil, pass `Pagination.Next` as `PaginationToken`, with the same filters
and `Limit`, to get the next page:

```go
var token *string

for {
	page, err := adminClient.User.List(ctx, &pinecone.ListUsersParams{PaginationToken: token})
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}
	for _, user := range page.Data {
		fmt.Println(user.Email)
	}
	if page.Pagination == nil {
		break
	}
	token = &page.Pagination.Next
}
```
