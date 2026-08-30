---
kind: note
level:
stage:
---
The gate machinery in this repository was built for a product that ships. It was then applied to a product that has never shipped, and the cost landed entirely on the sessions that write the product.

Two rules pulled in the same direction and compounded. One made every red a defect its finder owns. The other made a green gate a precondition of a commit. Together they turned a fixture drift into a session: find red, own red, repair red, re-run gate, find the next red. The product code that the session was opened to write is what got dropped.

The owner's ruling on 2026-08-30 ends that compounding by naming which of the two things is the deliverable. Nothing about product quality changes, because product quality was never what the sessions were spending on.
