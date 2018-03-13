# vault-read-aws2env
Read Vault ephemeral AWS creds into environment variables

## Usage

By defualt, Vault's AWS Secrets Backend generates output as a table like this
(or as a JSON blob):

```
$ vault read aws/creds/my-role
Key                Value
---                -----
lease_id           aws/creds/my-role/f3e92392-7d9c-09c8-c921-575d62fe80d8
lease_duration     768h
lease_renewable    true
access_key         AKIAIOWQXTLW36DV7IEA
secret_key         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
security_token     <nil>
```

You can't directly use this with tool like boto or awscli, so this tool just
converts it into shell commands to set your local environment variables.:

```
$ vault-read-aws2env aws/creds/my-role
export AWS_ACCESS_KEY_ID=AKIAIOWQXTLW36DV7IEA
export AWS_SECRET_ACCESS_KEY=iASuXNKcWKFtbO8Ef0vOcgtiL6knR20EJkJTH8WI
```

Making it suitable for sourcing!

```
$ $(vault-read-aws2env aws/creds/my-role)
```

If `security_token` is set, that will be exported as `AWS_SESSION_TOKEN`.


## Getting a Token

If you use `vault login` or `vault auth` to login to vault, this tool should
transparently pick that up for you. (and, I think it should also pick up token
helpers, but this hasn't been tested yet).


## Customization with Environment Variables

Standard Vault configuration should also work for this tool, specifically, you
can set the following env vars:

* `VAULT_TOKEN` - specifically override vault token
* `VAULT_ADDR` - point at a specific vault host

Other vault environment variables will probably work as well.


## Vault API compatibility

This is pinned against v0.9.5 release of Hashicorp's vault, notes on
(in)compatibilities are welcomed!


## This binary is *so* big!


Yeah... it is kinda huge. I think we could slim it down by more selectively
importing from hashicorp's vault library (or possibly by using a different
dependency tool). PRs welcome!
