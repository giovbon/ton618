FROM node:20-alpine
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm install --legacy-peer-deps
COPY web/ .
COPY internal/ ../internal/
RUN npx tailwindcss -c tailwind.config.cjs -i src/input.css -o static/app.css --minify
RUN cat static/app.css | wc -c
