---
kind: directive
level: MUST
stage:
---
1. You MUST cite every capability claim with a durable source.
   - You SHOULD prefer upstream source code links for implemented behavior.
   - You MUST use official feature documentation when code is not practical to cite.
   - For integrated products, you MUST cite both the wrapper/integration surface and the integrated project where the runtime behavior lives. Example: for VyOS routing features, cite VyOS config/templates and FRR documentation or source when FRR implements the protocol.
2. You MUST state the comparison scope before the matrix.
   - You MUST name the inspected checkout, release, branch, commit, docs page, or upstream feature page.
   - You MUST say that `not found` means not found in the inspected scope, not a universal absence claim.
3. You MUST label uncertainty instead of turning it into a gap.
   - You MUST use `Unclear` when evidence is incomplete.
   - You MUST use `Partial` when a narrower feature exists but is not equivalent to the compared feature.
   - You MUST separate similarly named features, such as IS-IS L1/L2 route leaking versus cross-VRF route leaking.
4. You MUST NOT cherry-pick categories to favor Ze.
   - You MUST include equivalent strengths from the other products.
   - If Ze is behind, you MUST say where it is behind and cite the evidence.
   - If another product delegates to an integrated daemon or OS facility, you MUST describe that delegation neutrally.
5. You MUST make wide comparison tables user-controllable.
   - Any comparison page with three or more product columns MUST provide controls to hide products the reader does not care about.
   - The controls MUST be keyboard-accessible and MUST NOT delete evidence from the source document.
