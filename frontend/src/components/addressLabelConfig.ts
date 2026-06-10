// Visibility-reason -> badge config for AddressLabel. Kept in its own module
// (not in AddressLabel.tsx) so the component file only exports a component
// (react-refresh/only-export-components) while tests can still import the
// mapping as the single source of truth and iterate Object.keys(LABEL_CONFIG).
export const LABEL_CONFIG: Record<string, { text: string; classes: string }> = {
  own_address:          { text: 'Mine',                 classes: 'bg-success-50 text-success-700' },
  rbac_group_member:    { text: 'My Org',               classes: 'bg-warning-50 text-warning-700' },
  public_address:       { text: 'Public',               classes: 'bg-neutral-100 text-neutral-500' },
  disclosure_grant:     { text: 'Disclosed',             classes: 'bg-purple-100 text-purple-700' },
  visible_to_grant:    { text: 'Shared',                classes: 'bg-blue-100 text-blue-700' },
  participant_override: { text: 'Counterparty',         classes: 'bg-primary-50 text-primary-700' },
};
