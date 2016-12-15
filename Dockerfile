FROM scratch

ENTRYPOINT ["/exposecontroller"]

COPY ./build/exposecontroller-linux-amd64 /exposecontroller
