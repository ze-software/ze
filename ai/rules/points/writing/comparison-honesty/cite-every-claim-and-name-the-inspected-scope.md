---
kind: directive
level:
stage:
---
1. Cite every capability claim with a durable source.
   - Prefer upstream source code links for implemented behavior.
   - Use official feature documentation when code is not practical to cite.
   - For integrated products, cite both the wrapper/integration surface and the integrated project where the runtime behavior lives. Example: for VyOS routing features, cite VyOS config/templates and FRR documentation or source when FRR implements the protocol.
2. State the comparison scope before the matrix.
   - Name the inspected checkout, release, branch, commit, docs page, or upstream feature page.
   - Say that `not found` means not found in the inspected scope, not a universal absence claim.
3. Label uncertainty instead of turning it into a gap.
   - Use `Unclear` when evidence is incomplete.
   - Use `Partial` when a narrower feature exists but is not equivalent to the compared feature.
   - Separate similarly named features, such as IS-IS L1/L2 route leaking versus cross-VRF route leaking.
4. Do not cherry-pick categories to favor Ze.
   - Include equivalent strengths from the other products.
   - If Ze is behind, say where it is behind and cite the evidence.
   - If another product delegates to an integrated daemon or OS facility, describe that delegation neutrally.
5. Make wide comparison tables user-controllable.
   - Any comparison page with three or more product columns must provide controls to hide products the reader does not care about.
   - The controls must be keyboard-accessible and must not delete evidence from the source document.
