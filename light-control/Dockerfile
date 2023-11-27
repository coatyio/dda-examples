# SPDX-FileCopyrightText: © 2023 Siemens AG
# SPDX-License-Identifier: MIT

#--------------------------------------------------
FROM    node:lts-alpine as install-base
#--------------------------------------------------

WORKDIR /opt/light-control

COPY    . .

## Install build toolchain, install node deps and compile native add-ons
## (required by node-gyp to compile @parcel/watcher on aarch64 platforms)
RUN     apk add --no-cache python3 make g++
RUN     npm clean-install

#--------------------------------------------------
FROM    node:lts-alpine as builder
#--------------------------------------------------

WORKDIR /opt/light-control

COPY   --from=install-base /opt/light-control/ .
RUN    npm run build

#--------------------------------------------------
FROM   nginx:stable-alpine as web-server
#--------------------------------------------------

COPY   --from=builder /opt/light-control/nginx.cors.conf /usr/share/nginx/html
COPY   --from=builder /opt/light-control/dist/light-control /usr/share/nginx/html

# Hosting port on Nginx server.
EXPOSE  80

# First, enable CORS for nginx with support for preflight requests (see https://enable-cors.org/server_nginx.html).
RUN    sed -i -e '/        try_files $uri $uri/r /usr/share/nginx/html/nginx.cors.conf' /etc/nginx/conf.d/default.conf

# Next, ensure routed Angular apps fallback to index.html
RUN    sed -i '/location \/ {/a\        try_files $uri $uri\/ \/index.html;' /etc/nginx/conf.d/default.conf

# Redirect output of HTTP client requests from console to /var/log/nginx/host.access.log
RUN    sed -i 's/#access_log/access_log/' /etc/nginx/conf.d/default.conf

CMD    ["nginx", "-g", "daemon off;"]
