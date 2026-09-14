| Test | Reason |
|------|--------|
| TestRFC5282CCMRefusesEveryOtherICVLength | Owner approved, 2026-09-14, as the same lint-only class approved earlier in this session. A bytes.Repeat of a zero byte becomes make, which gocritic asks for and which allocates the same forty zero octets. No assertion, input or outcome changes. RFC5282-3.2-4 is proven in both polarities exactly as before. |
