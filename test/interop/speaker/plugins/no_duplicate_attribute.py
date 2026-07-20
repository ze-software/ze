"""Test plugin: reject an UPDATE that carries the same path attribute more than once.

RFC 7606 Section 3(g): an attribute other than MP_REACH/MP_UNREACH appearing more than once
is handled by discarding all but the first -- but real implementations (FRR) go further and
treat the whole route as withdrawn, logging "attribute type N appears twice". Ze's route-server
replay once emitted NEXT_HOP twice; ze's OWN validator is lenient there, so only an independent,
strict peer catches it. That is this plugin's entire job.

This is deliberately the strict interpretation, not ze's lenient one -- an independent check is
worthless if it inherits ze's leniency.
"""

NAME = "no-duplicate-attribute"


def on_update(update, ctx):
    seen = set()
    for attr in update.attributes:
        if attr.code in seen:
            ctx.fail(
                "path attribute type %d appears more than once in one UPDATE"
                % attr.code
            )
        seen.add(attr.code)
