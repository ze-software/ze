---
kind: directive
level: MUST NOT
stage:
---
**A status, a state, a disposition and a severity are read by machines here. A value carrying the right word PLUS a human explanation is a different string, and a parser matching the whole field reads it as a different value. You MUST NOT put the explanation inside the field.**
**The failure is silent and it INVERTS the field's meaning:** the record says one thing and every gate reading it acts on the opposite. Explanation belongs beside the field, in an adjacent column or the prose around the table.
