FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -tags timetzdata -o /baro-backend ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /baro-backend /baro-backend
ENV ENVIRONMENT=production PORT=3000
EXPOSE 3000
CMD ["/baro-backend"]
