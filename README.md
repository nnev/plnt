## Overview

plnt is an RSS feed aggregator.

plnt is configured via a [TOML](https://github.com/toml-lang/toml) file (located
at `/etc/plnt.toml` by default) and spits out an HTML and ATOM feed.

## Caching

A representation of each feed is persisted to the cache directory whenever the
feed is accessed. Subsequent requests set the `If-Modified-Since` HTTP header
and will load the feed from cache if it has not changed on the server, or if the
server is unreachable (so that brief unavailability will not cause spurious
changes in the output).

For quick development cycles, you can use the `-force_from_cache` command line
flag to skip any network access.

It is safe to delete files (individually or all at once) from the cache
directory to force an update.
