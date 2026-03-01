FROM oven/bun:latest

WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --production

COPY src/ src/
COPY ui/dist/ ui/dist/

EXPOSE 9443 9080

CMD ["bun", "run", "src/index.ts"]
