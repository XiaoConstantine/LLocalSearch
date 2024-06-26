FROM node:20-alpine3.19 AS builder
ARG PUBLIC_VERSION="0.0.0"
ENV PUBLIC_VERSION=${PUBLIC_VERSION}
RUN echo "Building PUBLIC_VERSION ${PUBLIC_VERSION}"
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build
# RUN npm prune --production
EXPOSE 4173
CMD ["npm", "run", "preview", "--", "--host"]

# FROM node:20-alpine3.19
# WORKDIR /app
# COPY --from=builder /app/build build/
# COPY --from=builder /app/node_modules node_modules/
# COPY package.json .
# EXPOSE 3000
# ENV NODE_ENV=production
# CMD [ "node", "build" ]
