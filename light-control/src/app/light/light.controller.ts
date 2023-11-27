/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { Observable, BehaviorSubject } from "rxjs";

import { ComServiceClient } from "../api/com_grpc_web_pb";
import { ActionCorrelated, ActionResult, ActionResultCorrelated, SubscriptionFilter } from "../api/com_pb";
import { environment } from '../../environments/environment';
import {
    ColorRgba,
    createUuid,
    isValidColorRgba,
    Light,
    LightAck,
    LightContext,
    LightContextFilter,
    LightStatus,
    Uuid,
} from "../shared/light.model";

/**
 * A light controller manages a single light with its context and observes
 * Action requests for remote operations to change the light's status.
 *
 * For communicating light status changes to the associated Angular
 * `LightComponent`, and to receive responses, the controller provides an
 * observable and a confirmation callback method.
 */
export class LightController {

    private _light: Light;
    private _lightContext: LightContext;
    private _lightStatus: LightStatus;
    private _lightColorChangeSubject: BehaviorSubject<string>;
    private _lightColorChange$: Observable<string>;
    private _ddaClient: ComServiceClient;

    constructor(private options: any) {
        this._light = this.createLight();
        this._lightContext = this.createLightContext(this._light.id, this.options.light.building, this.options.light.floor, this.options.light.room);
        this._lightStatus = this.createLightStatus(this._light.id, this.options.light.on, this.options.light.color, this.options.light.luminosity);

        // Set initial color on the subject for emission when observers subscribe to it.
        this._lightColorChangeSubject = new BehaviorSubject<string>(this.convertLightStatusToColor());
        this._lightColorChange$ = this._lightColorChangeSubject.asObservable();
        this._ddaClient = new ComServiceClient(this.ddaEndpointUrl);

        // Start observing Action operations for light status control.
        this.observeActions();
    }

    /**
     * Gets the light object this controller is managing. The instance returned
     * is considered immutable, i.e. it will never change.
     */
    get light() {
        return this._light;
    }

    /**
     * Gets the light context object this controller is managing. The instance
     * returned is always the same, however, its properties will be modified
     * in-place by the `LightComponent` view on user interaction.
     */
    get lightContext() {
        return this._lightContext;
    }

    /**
     * Gets an observable on which change requests in light color are emitted by
     * this controller in response to light controlling remote operations or to
     * set the color for the initial light status.
     */
    get lightColorChange$() {
        return this._lightColorChange$;
    }

    /**
     * Gets the endpoint address URL for the DDA gRPC-Web API.
     */
    get ddaEndpointUrl() {
        return `${window.location.protocol}//${window.location.hostname}:${environment.grpcWebPort}`;
    }

    private observeActions() {
        this._ddaClient.subscribeAction(new SubscriptionFilter().setType(this.options.lightControlOperation))
            .on("data", ac => this.handleAction(ac))
            // Note: metadata with dda-suback acknowledgment is not emitted
            // until the server stream is closed; i.e. after data has been
            // received. You cannot receive metadata before stream data although
            // the HTTP response header contains it. This is an issue of the
            // grpc-web library.
            //.on("metadata", md => console.log("gRPC-Web Metadata:", md))
            .on("end", () => console.log("gRPC-Web End"))
            .on("status", status => console.log("gRPC-Web Status:", status))
            .on("error", err => console.error("gRPC-Web Error:", err));
    }

    private handleAction(ac: ActionCorrelated) {
        let params: any;
        try {
            params = JSON.parse(new TextDecoder("utf-8").decode(ac.getAction()!.getParams() as Uint8Array));
        } catch (error) {
            // Ignore Action whose parameters cannot be decoded.
            console.error("Error decoding Action params:", error)
            return;
        }

        const { on, color, luminosity, switchTime, contextFilter } = params;

        // Check if context filter matches this light's context.
        if (!this.matchesContextFilter(contextFilter)) {
            return;
        }

        // Respond with an InvalidParams error if parameter validation failed.
        if (!this.validateSwitchOpParams(on, color, luminosity, switchTime)) {
            this.publishActionResult(ac, "Invalid parameters")
            return;
        }

        // Respond with a custom error if the light is currently defect.
        if (this.light.isDefect) {
            this.publishActionResult(ac, "Light is defect")
            return;
        }

        // Respond with an acknowledgement that the light status is being
        // changed returning the current status before the change, and an
        // indication that the final result is still pending.
        this.publishActionResult(ac, undefined, this._lightStatus.on, 1)

        setTimeout(() => {
            this.updateLightStatus(on, color, luminosity);

            // Emit a light color change to trigger update of the LightComponent view.
            this._lightColorChangeSubject.next(this.convertLightStatusToColor());

            // Respond with a result indicating whether the light is in
            // state on or off, and that it is the final result.
            this.publishActionResult(ac, undefined, this._lightStatus.on, -2);
        },
            // Ensure timeout is greater than 0, otherwise the final action result
            // with sequence number -2 is not guaranteed to be transmitted on the
            // wire AFTER the first action result with sequence number 1, especially
            // if HTTPS is used.
            Math.max(1, switchTime === undefined ? 0 : switchTime));
    }

    private publishActionResult(ac: ActionCorrelated, error?: string, isLightOn?: boolean, seqNum: number = 0) {
        this._ddaClient.publishActionResult(new ActionResultCorrelated()
            .setCorrelationId(ac.getCorrelationId())
            .setResult(new ActionResult()
                .setContext("lightcontrol.light.controller")
                .setSequenceNumber(seqNum)
                .setData(new TextEncoder().encode(JSON.stringify((error ? {
                    error,
                    executionInfo: { lightId: this.light.id, triggerTime: Date.now() },
                } : {
                    isLightOn,
                    executionInfo: { lightId: this.light.id, triggerTime: Date.now() },
                }) as LightAck)))
            ),
            undefined,
            err => {
                if (err) {
                    console.error("Error publishing ActionResult:", error);
                }
            }
        );

    }

    private createLight(): Light {
        return {
            id: createUuid(),
            isDefect: false,
        };
    }

    private createLightStatus(lightId: Uuid, on: boolean, color: ColorRgba, luminosity: number): LightStatus {
        return {
            on,
            luminosity,
            color,
        };

    }

    private createLightContext(lightId: Uuid, building: number, floor: number, room: number): LightContext {
        return {
            lightId,
            building,
            floor,
            room,
        };
    }

    private matchesContextFilter(cf: LightContextFilter): boolean {
        if (!cf) {
            return false;
        }
        if (cf.lightId !== undefined && (typeof cf.lightId === "string")) {
            return cf.lightId === this._light.id;
        }
        if (Array.isArray(cf.buildings) && Array.isArray(cf.floors) && Array.isArray(cf.rooms)) {
            return cf.buildings.includes(this._lightContext.building!) &&
                cf.floors.includes(this._lightContext.floor!) &&
                cf.rooms.includes(this._lightContext.room!);
        }
        return false;
    }

    private validateSwitchOpParams(on?: boolean, color?: ColorRgba, luminosity?: number, switchTime?: number) {
        // Validate operation parameters, return undefined if validation fails.
        if ((on === undefined || typeof on === "boolean") &&
            (luminosity === undefined || (typeof luminosity === "number" && luminosity >= 0 && luminosity <= 1)) &&
            (switchTime === undefined || (typeof switchTime === "number")) &&
            isValidColorRgba(color) &&
            // For testing purposes, yield an error if color is black.
            !(color![0] === 0 && color![1] === 0 && color![2] === 0)) {
            return true;
        }
        return false;
    }

    private updateLightStatus(on?: boolean, color?: ColorRgba, luminosity?: number) {
        const { on: currentOn, color: currentColor, luminosity: currentLuminosity } = this._lightStatus;
        const id = this._light.id;
        if (on === undefined || on === currentOn) {
            if (color === undefined || color === currentColor) {
                if (luminosity === undefined || luminosity === currentLuminosity) {
                    // Nothing changed
                } else {
                    this._lightStatus = this.createLightStatus(id, currentOn, currentColor, luminosity);
                }
            } else {
                if (luminosity === undefined || luminosity === currentLuminosity) {
                    this._lightStatus = this.createLightStatus(id, currentOn, color, currentLuminosity);
                } else {
                    this._lightStatus = this.createLightStatus(id, currentOn, color, luminosity);
                }
            }
        } else {
            if (color === undefined || color === currentColor) {
                if (luminosity === undefined || luminosity === currentLuminosity) {
                    this._lightStatus = this.createLightStatus(id, on, currentColor, currentLuminosity);
                } else {
                    this._lightStatus = this.createLightStatus(id, on, currentColor, luminosity);
                }
            } else {
                if (luminosity === undefined || luminosity === currentLuminosity) {
                    this._lightStatus = this.createLightStatus(id, on, color, currentLuminosity);
                } else {
                    this._lightStatus = this.createLightStatus(id, on, color, luminosity);
                }
            }
        }
    }

    /**
     * Convert the current light status object to a CSS rgb/rgba color string.
     */
    private convertLightStatusToColor(): string {
        const { on, color, luminosity } = this._lightStatus;
        if (!on || luminosity === 0) {
            return "transparent";
        }
        return `rgba(${color[0]}, ${color[1]}, ${color[2]}, ${(luminosity * color[3]).toFixed(2)})`;
    }

}
