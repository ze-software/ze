| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | tests | `net.Pipe()` write blocked because no goroutine was reading | started reader goroutine before write |
| 2026-03-21 | - | tests | same deadlock in a different test using sequential write-then-read | wrapped write in its own goroutine |
