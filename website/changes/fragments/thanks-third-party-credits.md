---
spec: thanks-third-party-credits
topic: CLI
feature: third-party-credits
---

## Before

Ze is built on other people's work, and nothing you could run said so. The
binary carried no licence text of the libraries compiled into it, and the
website's Dependencies page listed what we use and why, but not under which
licence.

## After

`ze thanks` lists every third-party work compiled into the binary you run:
Go itself, each Go module the build links, the web assets, and the code we
copied or ported from other projects. Each row gives the version this binary
carries, the licence, the copyright holder, why we use it and where its
source is. A build without a feature does not list what that feature brings:
a binary without the web interface, the looking glass and the REST server
lists no web asset.

`ze thanks licence` prints the licence and notice texts the binary carries,
each as its work ships it. The daemon answers the same as `show thanks list`
and `show thanks licence`, so SSH, the web command pages, REST, gRPC and MCP
reach them too, and `show thanks licence | name github.com/miekg/dns`
narrows the texts to one work.

The REST documentation page now serves the swagger-ui licence, its notice and
the licence list of the libraries in its bundle, behind the same
authentication as the page. The web interface and the looking glass serve the
htmx and uPlot licences beside the assets.

On the website, the Dependencies page shows the licence and copyright holder
of each module, and a new section, "Also in the shipped binary", lists every
other work the binary carries.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
