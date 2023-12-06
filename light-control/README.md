# DDA Example - Light Control Web App

[![Powered by DDA](https://img.shields.io/badge/Powered%20by-DDA-00ADD8.svg)](https://github.com/coatyio/dda)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/coatyio/dda-examples/blob/main/LICENSE)

## Table of Contents

* [Introduction](#introduction)
* [User Interface](user-interface)
* [Quick Start](#quick-start)
* [How It Works](#how-it-works)
* [Developer Notes](#developer-notes)
* [License](#license)

## Introduction

This example project provides an [Angular](https://angular.io) web app that
demonstrates how remote operations exposed by a [Data Distribution
Agent](https://github.com/coatyio/dda) (DDA) sidecar can be used to switch
multiple distributed light sources by decentralized lighting control units.
Remote light switching operations are context-filtered to enable control of
individual lights or groups of lights in specific rooms, on specific floors, or
in specific buildings.

The web app uses the public gRPC-Web API of its associated DDA sidecar to
control light sources by issuing remote operations with the two-way Action
communication pattern.

This example includes a ready-to-use deployment that runs with Docker.

## User Interface

After opening the light control UI, create several lights by clicking the "NEW
LIGHT" button in the action bar. Each light is opened in its own popup window.
Using the sliders, you can configure a light's context, which indicates where
the light should be physically located (i.e. building, floor, and room).

> __TIP__: If your browser has popups disabled, you can launch a new light UI in
> a new browser tab by clicking the "NEW LIGHT AS TAB" button in the action bar.

Now, you can control a specific set of lights or individual lights in the light
control UI by

* selecting appropriate context filter settings that define matching lights,
* selecting action parameters for the matching lights (i.e. on/off state,
  luminosity, color, switch time).

Perform the selected action with the selected filter by clicking the "SWITCH
LIGHTS" button.

> __TIP__: To execute the action immediately whenever you change an action
> parameter, check the checkbox next to the "SWITCH LIGHTS" button.

Click the fab button "{}" to view the Action details for the currently selected
parameters and context filter.

The expandable action log view provides details on the published Actions and
received ActionResults. Click a code fab button "{}" to view the details of a
specific action. Note that any errorless action is responded by two results: the
first one is sent as an acknowledgment returning the current status of the light
_before_ the change; the second one is sent _after_ the light has been switched
successfully returning the new status of the light.

To force errors to be returned by an action result, turn on the "Light defect"
switch in the light UI or select the invalid "black" color from the color
palette in the light control UI. In this case, only _one_ result is returned
indicating the error.

> __TIP__: You can also control an individual light by dragging the QR Code
> displayed in the light UI and dropping it onto the corresponding area in the
> context filter panel. Now, the operation context is limited to the selected
> light. Building, floor, and room filters are disabled and ignored. To enable
> these filters again, remove the QR Code from the context filter by clicking
> the "clear" button.
>
> __TIP__: Alternatively, you can limit the context filter to a specific light
> by clicking on the light's QR Code or by scanning the QR Code (with your
> mobile device). On both cases, a new light control UI is opened with the
> corresponding operation context.
>
> __TIP__: You can also start multiple light control UIs simultaneously to
> demonstrate a decentralized lighting control system by clicking the "NEW LIGHT
> CONTROL" button in the action bar.

## Quick Start

Make sure you have [Docker Engine](https://www.docker.com/) installed on your
target host system.

Download the light-control project from the project
[repository](https://github.com/coatyio/dda-examples/grpc-web/light-control/tree/main/quick-start).

Use `docker compose up` from within the `quick-start` folder to start up the
light control application including a web app server, a DDA sidecar, and an MQTT
5 broker. Use `docker compose down` to shut down the services.

Launch the web app in a browser pointed at the target host on port `8098`.

> __NOTE__: To use this example with a new DDA version released on GitHub,
> download any updated light-control project _and_ delete the outdated local
> Docker containers `quick-start-webserver` and `ghcr.io/coatyio/dda:latest`
> before running `docker conmpose up` again.
>
> __NOTE__: To enable HTTPS over the web app's gRPC-Web connection to the DDA
> sidecar create a valid server certificate and add its private key file and its
> certificate file to the `quick-start` folder in PEM format. Then, reference
> these files in the `dda.yaml` file in the `apis/cert` and `apis/key` section
> using the absolute path `/dda/<name of key/cert file>.pem`.

## How It Works

This project is a single-page web application that was built with
[Angular](https://angular.io) and generated with [Angular
CLI](https://angular.io/cli).

The Angular single-page web app consists of two separate lazy loaded Angular
modules representing either a single light UI or a light control UI. These pages
are accessible on separate routes (`/light`, and `/control` (default)). When the
app starts up, depending on the given route, only the associated module is
loaded.

To switch lights, light UIs and light control UIs utilize the communication
service of the associated DDA sidecar to communicate with each other using the
two-way Action communication pattern. The web UIs use the public gRPC-Web API
exposed by the sidecar. The client-side gRPC-Web API JavaScript code (together
with TypeScript type definitions) has been imported into this project from the
DDA sources (see folder `src/app/api/` imported from DDA folder
`apis/grpc/stubs/js`). More information on the gRPC-Web JavaScript
implementation for browser clients can be found
[here](https://github.com/grpc/grpc-web).

Starting up a new light UI invokes the server streaming API function
`subscribeAction` to receive remote light control operations triggered by a
light control UI (see method `LightController.observeActions`). Whenever such an
operation is received (see method `LightController.handleAction`) its parameters
are decoded and the context filter is matched against the context of the given
light UI (i.e. its building, floor, room). If both match, the light UI status is
changed and two ActionResults are published by invoking the unary API function
`publishActionResult`, one before the change (as an acknowledgment) and one
after the change, encoding the new status and additional execution information
in its data. Note that Action parameters should always be validated. If Action
parameters cannot be decoded the action is ignored and not responded. If Action
parameters are invalid, or if the light is defect, only one ActionResult is
published encoding the corresponding error in its data. Also note that
acknowledgments for unary function `publishActionResult` should be awaited
before publishing another result so as to ensure sequential transmission.

A light control UI invokes light switching operations using the server streaming
API function `publishAction` (see method `ControlController.switchLights`). Note
that a deadline is set as gRPC-Web metadata to ensure that the triggered HTTP
POST request is canceled eventually after ActionResults have been received. If
no deadline were set each HTTP action request would block its associated HTTP
connection, preventing other requests from being sent on it. As the number of
HTTP connections is strongly limited by a browser this situation would block the
web app completely. ActionResult data received by the `publishAction` callback
`on("data")` is decoded and added to the correlated action in the action log
view. Note that result data should always be validated. If data cannot be
decoded the result is ignored.

The local data flow between a controller (`LightController`,
`ControlController`) and its corresponding Angular view component
(`LightComponent`, `ControlComponent`) is implemented using RxJS observables.
Observables can be efficiently handled inside Angular view templates using the
`async` pipe.

Light control configuration parameters are contained in a central config file
named `light-control.config.json` located in the `src/assets/config/` folder of
the project. This configuration contains options which are exposed by the light
UI and the light control UI. It is retrieved via a HTTP GET request from the web
server hosting the app (see class `LightControlConfigService`).

The HTTP port of the DDA gRPC-Web server endpoint address is configured in the
environment file `src/environments/environment.ts` and accessed by the getter
methods `ddaEndpoint` of classes `LightController` and `ControlController`. As
the DDA sidecar is hosted along with the web server which serves the web app the
endpoint hostname is identical to the hostname of the web server.

## Developer Notes

For development, make sure that an actual long-term-stable `Node.js` JavaScript
runtime (LTS version 18.16.1 or later) is globally installed on your target
machine. Download and installation details can be found
[here](http://nodejs.org/).

Install [Angular CLI](https://angular.io/cli) by `npm install -g @angular/cli`.
Note that this project was generated with Angular CLI version 16.2.5.

Checkout the example sources from
[here](https://github.com/coatyio/dda-examples/grpc-web/light-control)
and install dependencies by `npm install`.

Ensure an MQTT 5 broker is available locally, listening on TCP port 1883. For
example, you may use [emqx](https://www.emqx.io/) or
[mosquitto](https://mosquitto.org/), like this:

```sh
# Run emqx on TCP port 1883
docker run --name emqx -p 1883:1883 emqx/emqx:latest

# Run mosquitto on TCP port 1883 with localhost non-TLS configuration
docker run -it -p 1883:1883 eclipse-mosquitto mosquitto -c /mosquitto-no-auth.conf
```

Next, start a DDA sidecar (with default configuration settings), either as an
executable or as a Docker container:

```sh
# Run DDA sidecar as binary available in the GitHUb release assets of
# the DDA project on https://github.com/coatyio/dda/releases
dda

# Run DDA sidecar as container available in the GitHub Container
# Registry of the DDA project on https://ghcr.io/coatyio/dda
docker run --rm -p 8800:8800 ghcr.io/coatyio/dda:latest
```

Finally, run `npm start` and open a browser with the light control UI on
`http://localhost:4200`.

To test the light control UI locally using gRPC-Web over _HTTPS_, run `npm run
start:https` and configure a valid server certificate for the DDA sidecar as
explained in the section [Quick Start](#quick-start). For testing on localhost
you may use a self-signed server certificate if your browser supports it. In
Chrome, enable this mode by setting the flag
`chrome://flags/#allow-insecure-localhost`.

## License

Code and documentation copyright 2023 Siemens AG.

Code is licensed under the [MIT License](https://opensource.org/licenses/MIT).

Documentation is licensed under a
[Creative Commons Attribution-ShareAlike 4.0 International License](http://creativecommons.org/licenses/by-sa/4.0/).
