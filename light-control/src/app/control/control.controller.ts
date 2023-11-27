/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { BehaviorSubject, Observable } from 'rxjs';

import { ComServiceClient } from "../api/com_grpc_web_pb";
import { Action, ActionResult } from "../api/com_pb";
import { environment } from '../../environments/environment';
import { ColorRgba, createUuid, LightContextFilter, Uuid } from '../shared/light.model';
import { ClientReadableStream, StatusCode } from 'grpc-web';

/**
 * Represents a data structure that holds an Action and its correlated
 * ActionResults together with timestamps for logging purposes.
 */
export interface ActionLogEntry {

    /**
     * The DDA Action to be logged. Note that the action's ID is used to
     * correlate a DDA ActionResult with it.
     */
    action: Action;

    /**
     * The action parameters as a plain object. Used as a cache to speed up
     * access from within component views.
     */
    actionParams: any;

    /** The timestamp the Action has been published. */
    actionTime: number;

    /**
     * An array of tuples of ActionResults with decoded result data as a plain
     * object and associated result timestamps ordered by time of arrival.
     */
    actionResults: Array<{ result: ActionResult; resultData: any; resultTime: number; }>;

    /**
     * Error message in case the associated action fails to be published
     * (optional).
     */
    error?: string;
}

/**
 * A light control controller invokes remote operations to control lights and
 * manages all invocations in a log.
 */
export class ControlController {

    private _id: Uuid;
    private _nextActionId: Uuid;
    private _actionLog: Array<ActionLogEntry>;
    private _actionLogSubject: BehaviorSubject<Array<ActionLogEntry>>;
    private _actionLog$: Observable<Array<ActionLogEntry>>;
    private _ddaClient: ComServiceClient;

    constructor(public options: any) {
        this._id = createUuid();
        this._nextActionId = createUuid();
        this._actionLog = [];
        this._actionLogSubject = new BehaviorSubject<Array<ActionLogEntry>>(this._actionLog);
        this._actionLog$ = this._actionLogSubject.asObservable();
        this._ddaClient = new ComServiceClient(this.ddaEndpointUrl);
    }

    /**
     * Gets the unique ID of this controller.
     */
    get id() {
        return this._id;
    }

    /**
     * Gets an observable on which action log changes are emitted by this
     * controller in response to light controlling remote operations.
     */
    get actionLog$() {
        return this._actionLog$;
    }

    /**
     * Gets the endpoint address URL for the DDA gRPC-Web API.
     */
    get ddaEndpointUrl() {
        return `${window.location.protocol}//${window.location.hostname}:${environment.grpcWebPort}`;
    }

    /**
     * Gets the source of any action published by this controller.
     */
    get actionSource() {
        return this._id;
    }

    /**
     * Gets the action ID for the next action to be published.
     */
    get nextActionId() {
        return this._nextActionId;
    }

    /**
     * Clear all entries in the action log.
     */
    clearActionLog() {
        this._actionLog = [];
        this._actionLogSubject.next(this._actionLog);
    }

    switchLights(contextFilter: LightContextFilter, onOff: boolean, luminosity: number, rgba: ColorRgba, switchTime: number) {
        const params = {
            on: onOff,
            color: rgba,
            luminosity,
            switchTime,
            contextFilter,
        };
        const action = new Action()
            .setType(this.options.lightControlOperation)
            .setId(this._nextActionId)
            .setSource(this.actionSource)
            .setParams(new TextEncoder().encode(JSON.stringify(params)));
        this._nextActionId = createUuid();
        const entry = this.addActionToActionLog(action, params);
        this._ddaClient.publishAction(action, {
            // In case no light control UI is present, provide a tight deadline
            // so that the stream is closed asap. If the browser uses HTTP/1.1
            // for grpc-web requests this is essential as each unary or server
            // streaming call is invoked on a separate HTTP connection. By
            // closing the stream the connection becomes available for other
            // calls, preventing the browser's maximum HTTP connection limit to
            // be reached (e.g. max 6 connections per domain, max 10 total in
            // Chrome). Note that this limitation doesn't exist if the browser
            // uses HTTP/2 as it supports multiplexing multiple grpc-web calls
            // on the same connection.
            //
            // Note that a DEADLINE_EXCEEDED error papers over any other timeout
            // error that takes longer to be triggered, such as
            // net::ERR_CONNECTION_REFUSED.
            deadline: (Date.now() + switchTime + 1500).toString(),
        })
            .on("data", result => this.addResultToActionLog(result, Date.now(), entry))

            // Note: header metadata with dda-suback acknowledgment is not
            // emitted until the server stream is closed; i.e. after deadline
            // expires. You cannot receive metadata as header data before
            // response data is delivered although the HTTP response header
            // contains it. This is an issue with the grpc-web library. If you
            // want to make use of dda-suback metadata in your app, consider
            // using the @improbable-eng/grpc-web library as an alternative. It
            // supports header metadata to be received before any response data.
            //
            //.on("metadata", md => console.log("gRPC-Web Metadata:", md))

            .on("end", () => { })
            //.on("status", status => console.log("gRPC-Web Status:", status, action.getId(), params))
            .on("error", err => {
                // Indicate deadline exceeded errors only if no results have been received yet.
                if (err.code !== StatusCode.DEADLINE_EXCEEDED || entry.actionResults.length === 0) {
                    console.error("gRPC-Web Error:", err, action.getId(), params);
                    entry.error = err.toString();
                }
            });
    }

    private addActionToActionLog(action: Action, actionParams: any) {
        const entry: ActionLogEntry = {
            action,
            actionParams,
            actionTime: Date.now(),
            actionResults: [],
        };
        this._actionLog.unshift(entry);
        this._actionLogSubject.next(this._actionLog);
        return entry;
    }

    private addResultToActionLog(result: ActionResult, resultTime: number, entry: ActionLogEntry) {
        let data: any;
        try {
            data = JSON.parse(new TextDecoder("utf-8").decode(result.getData() as Uint8Array));
        } catch (error) {
            // Ignore ActionResult whose parameters cannot be decoded.
            console.error("Error decoding ActionResult:", error);
            return;
        }
        entry.actionResults.push({
            result,
            resultData: data,
            resultTime,
        });
        this._actionLogSubject.next(this._actionLog);
    }
}
