FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache bash ca-certificates tzdata
COPY sun-panel /app/sun-panel
COPY entrypoint.sh /app/entrypoint.sh
COPY conf.example.ini /app/conf.example.ini
RUN chmod +x /app/sun-panel /app/entrypoint.sh \
    && mkdir -p /app/conf /app/database /app/uploads /app/runtime/runlog
EXPOSE 3002
CMD ["/app/entrypoint.sh"]
