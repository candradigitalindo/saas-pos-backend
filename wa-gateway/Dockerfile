# Sidecar WhatsApp. Sengaja TIDAK memakai multi-stage: tidak ada langkah build,
# kode dijalankan apa adanya oleh Node.
FROM node:22-alpine

WORKDIR /app

# Dipasang lebih dulu supaya lapisan dependensi bisa di-cache terpisah dari kode.
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev --no-audit --no-fund || npm install --omit=dev --no-audit --no-fund

COPY src/ ./src/

# Kredensial WhatsApp harus hidup lebih lama dari kontainer — tanpa volume di
# sini, setiap restart meminta pindai QR ulang.
VOLUME ["/app/auth"]
ENV WA_AUTH_DIR=/app/auth \
    WA_GATEWAY_HOST=0.0.0.0 \
    WA_GATEWAY_PORT=8090
EXPOSE 8090

# Jangan jalan sebagai root; folder auth diserahkan ke user node.
RUN mkdir -p /app/auth && chown -R node:node /app
USER node

CMD ["node", "src/index.js"]
