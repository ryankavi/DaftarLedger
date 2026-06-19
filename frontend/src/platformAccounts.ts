// Fixed ids of the platform accounts seeded by migration
// 000007_seed_platform_accounts. They're stable across deploys, so the UI can
// reference them directly instead of looking them up.
// Values are typed as plain `string` (no `as const`) so useState(...) infers a
// `string` state rather than a narrow string-literal type — otherwise the
// field's onChange (setX(e.target.value)) would fail to type-check.
export const PLATFORM_ACCOUNTS: Record<string, string> = {
  EXTERNAL: 'a0000000-0000-0000-0000-000000000001',
  TREASURY: 'a0000000-0000-0000-0000-000000000002',
  FEE_REVENUE: 'a0000000-0000-0000-0000-000000000003',
  CARD_SETTLEMENT: 'a0000000-0000-0000-0000-000000000004',
  ACH_CLEARING: 'a0000000-0000-0000-0000-000000000005',
}
