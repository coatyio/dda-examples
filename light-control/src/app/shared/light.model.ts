/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { v4 as uuidv4 } from 'uuid';

/**
 * Type alias for RFC4122 V4 UUIDs represented as strings.
 */
export declare type Uuid = string;

/**
 * Creates a unique UUID v4.
 * @returns a unique UUID v4 as a string
 */
export function createUuid(): Uuid {
    return uuidv4()
}

/**
 * Defines a color tuple array with RGBA values.
 */
export interface ColorRgba extends Array<number> {
    1: number;  // Red 0..255
    2: number;  // Green 0..255
    3: number;  // Blue 0..255
    4: number;  // Alpha 0..1
}

export function isValidColorRgba(color: ColorRgba | undefined) {
    return Array.isArray(color) && color.length === 4 &&
        color[0] >= 0 && color[0] <= 255 &&
        color[1] >= 0 && color[1] <= 255 &&
        color[2] >= 0 && color[2] <= 255 &&
        color[3] >= 0 && color[3] <= 1;
}

/**
 * Models a lighting source which can change color and adjust luminosity.
 */
export interface Light {
    /**
     * The unqiue ID of this light.
     */
    id: Uuid;

    /**
     * Determines whether the light is currently defect. The default value is
     * `false`.
     */
    isDefect: boolean;
}

/**
 * Models the current status of a light including on-off, color-change and
 * luminosity-adjust features.
 */
export interface LightStatus {
    /**
     * Determines whether the light is currently switched on or off.
     */
    on: boolean;

    /** The current luminosity level of the light, a number between 0 (0%) and 1
     * (100%).
     */
    luminosity: number;

    /**
     * The current color of the light as an rgba tuple.
     */
    color: ColorRgba;
}

/**
 * Represents the environmental context of a light. The light context defines a
 * building number, a floor number, and a room number indicating where the light
 * is physically located. To control an individual light, the light's ID is also
 * part of the context. In this case the other properties should not be
 * specified.
 */
export interface LightContext {
    // Associated light Id, if any (optional).
    lightId?: Uuid;

    // The number of the building in which this light is located.
    building?: number;

    // The number of the floor on which the light is located.
    floor?: number;

    // The number of the room on which the light is located.
    room?: number;
}

/**
 * Defines filter criteria to match against the context of a light.
 */
export interface LightContextFilter {
    lightId?: Uuid;
    buildings?: number[];
    floors?: number[];
    rooms?: number[];
}

/**
 * Determines whether the given light context matches the given context filter.
 * @param ctx a light context
 * @param filter a filter for a light context
 * @returns true if context and filter match; false otherwise
 */
export function matchesLightContext(ctx: LightContext, filter: LightContextFilter) {
    if (ctx.lightId == undefined && filter.lightId == undefined) {
        return filter.buildings!.includes(ctx.building!) &&
            filter.floors!.includes(ctx.floor!) &&
            filter.rooms!.includes(ctx.room!);

    }
    if (ctx.lightId != undefined && filter.lightId != undefined) {
        return filter.lightId == ctx.lightId;
    }
    return false;
}

/**
 * Defines the structure of the ranges of LightContext properties. To be used by
 * UI components for input validation and restriction.
 *
 * The concrete ranges are defined in 'assets/config/light-control.config.json'.
 */
export interface LightContextRanges {
    building: { min: number, max: number, tickInterval: number };
    floor: { min: number, max: number, tickInterval: number };
    room: { min: number, max: number, tickInterval: number };
}

/**
 * Represents execution information returned with a light control
 * action result.
 */
export interface LightExecutionInfo {

    /** Id of the Light that has been controlled. */
    lightId: Uuid;

    /**
     * The timestamp in UTC milliseconds when the light control operation has
     * been triggered.
     */
    triggerTime: number;
}

/**
 * Represents the result data returned with a light control action result.
 */
export interface LightAck {
    /**
     * Contextual Execution information.
     */
    executionInfo: LightExecutionInfo;

    /**
     * Error message (optional).
     */
    error?: string;

    /**
     * Whether light has been switched on or off (optional).
     */
    isLightOn?: boolean;
}
