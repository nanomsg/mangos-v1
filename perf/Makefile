#
# Makefile to buld nanomsg compatible tests
#
GO = go
RM = rm -f
LINKS = local_lat remote_lat local_thr remote_thr inproc_thr inproc_lat \
	remote_reqlat local_reqlat

all:	perf $(LINKS)

clean:
	$(RM) $(LINKS) perf

perf:
	$(GO) build .

$(LINKS):
	$(RM) $@
	ln -s perf $@

