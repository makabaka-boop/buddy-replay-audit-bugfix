FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /allocator .

FROM alpine:3.20
COPY --from=build /allocator /allocator
EXPOSE 8080
CMD ["/allocator"]
