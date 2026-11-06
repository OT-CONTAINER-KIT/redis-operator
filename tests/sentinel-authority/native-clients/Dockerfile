FROM node:22-alpine
WORKDIR /probe
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --omit=dev --no-audit --no-fund
COPY probe.cjs ./
RUN npm test
USER node
ENTRYPOINT ["node", "/probe/probe.cjs"]
