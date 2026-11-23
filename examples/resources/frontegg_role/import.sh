# A vendor-level role is imported by its ID.
terraform import frontegg_role.example <role_id>

# A tenant role is imported as <tenant_id>:<role_id>.
terraform import frontegg_role.example <tenant_id>:<role_id>
