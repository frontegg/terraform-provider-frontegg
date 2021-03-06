Thank you for opening an issue. Please note that we try to keep the our issue tracker reserved for bug reports and feature requests. For general usage questions, please see: https://www.terraform.io/community.html.

### Terraform version

Run `terraform -v` to show the version. If you are not running the latest
version of Terraform, please upgrade because your issue may have already been
fixed.

### Affected resources

Please list the resources as a list, for example:

* opc_instance
* opc_storage_volume

If this issue appears to affect multiple resources, it may be an issue with
Terraform's core, so please mention this.

### Terraform configuration files

```hcl
# Copy-paste your Terraform configurations here - for large Terraform configs,
# please use a service like Dropbox and share a link to the ZIP file.
```

### Debug Output

Please provider a link to a GitHub Gist containing the complete debug output: https://www.terraform.io/docs/internals/debugging.html. Please do NOT paste the debug output in the issue; just paste a link to the Gist.

### Panic output

If Terraform produced a panic, please provide a link to a GitHub Gist containing the output of the `crash.log`.

### Expected Behavior

What should have happened?

### Actual Behavior

What actually happened?

### Steps to reproduce

Please list the steps required to reproduce the issue, for example:

1. `terraform apply`

### Important factoids

Are there anything atypical about your accounts that we should know? For example: Running in EC2 Classic? Custom version of OpenStack? Tight ACLs?

### References

Are there any other GitHub issues (open or closed) or pull requests that should
be linked here? For example:

- GH-1234
