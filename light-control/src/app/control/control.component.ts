/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import {
    AfterContentInit,
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    OnDestroy,
    OnInit,
    ViewEncapsulation,
    ViewChild,
    ElementRef,
} from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonToggleChange } from '@angular/material/button-toggle';
import { MatSlideToggleChange } from '@angular/material/slide-toggle';
import { MatSliderDragEvent } from '@angular/material/slider';
import { Observable, Subject } from 'rxjs';
import { filter, map } from 'rxjs/operators';

import { AppContextService } from '../app-context.service';
import { ColorRgba, LightContextFilter, LightContextRanges, Uuid } from '../shared/light.model';
import { ControlController, ActionLogEntry } from './control.controller';
import { CodeViewerBottomSheetComponent } from './code-viewer-bottom-sheet.component';
import { LightControlConfigService } from '../shared/light-control-config.service';
import { Action } from '../api/com_pb';

interface WindowLayout {
    screenLeft: number;
    screenTop: number;
    outerWidth: number;
    outerHeight: number;
    availTop: number;
    availLeft: number;
    availWidth: number;
    availHeight: number;
}

@Component({
    selector: 'app-control',
    templateUrl: 'control.component.html',
    styleUrls: ['control.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,

    // Allow Material styles to be overriden in component style.
    // Needed for custom theming of color slider.
    encapsulation: ViewEncapsulation.None,
})
export class ControlComponent implements AfterContentInit, OnDestroy, OnInit {

    @ViewChild('controlCard', { read: ElementRef, static: true }) cardElementRef!: ElementRef;

    controller?: ControlController;
    connectionInfo?: string

    selectedBuildings: number[] = [];
    selectedFloors: number[] = [];
    selectedRooms: number[] = [];
    selectedLightId?: Uuid;
    selectedLightUrl?: string;

    availableBuildings: number[] = [];
    availableFloors: number[] = [];
    availableRooms: number[] = [];

    autoSwitch = false;

    // ngModel bindings for operation parameters.
    onOff: boolean = false;
    luminosityPercent: number = 100;            // 0..100
    primaryColorPosition: number = 0;         // 0..<primaryColorPositionMax> or -1
    customColors: Array<{ name: string, rgba: ColorRgba }> = [];
    selectedCustomColor?: { name: string, rgba: ColorRgba };
    switchTime: number = 0;

    readonly primaryColorPositionMax = 1000;

    actionLog$?: Observable<Array<ActionLogEntry>>;

    readonly currentClock$ = new Subject<number>();
    isClockStopped: boolean = true;

    private currentLightWindowLayout?: WindowLayout;

    constructor(
        private appContext: AppContextService,
        private bottomSheet: MatBottomSheet,
        private changeRef: ChangeDetectorRef,
        private route: ActivatedRoute,
        private config: LightControlConfigService
    ) {
        this.appContext.setContext('Light Control');
        this.startClock();
        this.initControlController();
    }

    /* Event handler */

    ngOnInit() {
        // Capture the query param 'light_id' if available to set the light context filter.
        this.route
            .queryParamMap
            .pipe(
                map(params => params.get('light_id')),
                filter(value => !!value)
            )
            .subscribe(lightId => {
                setTimeout(() => {
                    this.selectedLightId = lightId || undefined;
                    this.selectedLightUrl = window.location.href;
                });
            });
    }

    ngAfterContentInit() {
        // The native slider element is available in the DOM not until the next
        // macrotask.
        setTimeout(() => {
            this.updateColorSliderThumb(this.primaryColorPosition);
        }, 100);
    }

    ngOnDestroy() {
        this.stopClock();
    }

    onOnOffToggle(event: MatSlideToggleChange) {
        if (this.autoSwitch) {
            this.switchLights();
        }
    }

    onLuminosityChange(value: number) {
        if (this.autoSwitch) {
            this.switchLights();
        }
    }

    primaryColorThumbDisplayer(position: number) {
        // Suppress slider thumb label.
        return '';
    }

    onPrimaryColorChange(value: number) {
        this.updateColorSliderThumb(value);
        this.selectedCustomColor = undefined;
        if (this.autoSwitch) {
            this.switchLights();
        }
    }

    onCustomColorChange(event: MatButtonToggleChange) {
        if (event.source.checked) {
            this.updateColorSliderThumb(-1);
            if (this.autoSwitch) {
                this.switchLights();
            }
        }
    }

    onSwitchTimeChange(event: MatButtonToggleChange) {
        if (this.autoSwitch) {
            this.switchLights();
        }
    }

    onQrCodeDragOver(event: DragEvent) {
        if (event.dataTransfer?.types.includes('text/qrcode')) {
            event.preventDefault();
        }
    }

    onQrCodeDrop(event: DragEvent) {
        const url = event.dataTransfer?.getData('text/qrcode');
        if (!url) {
            return;
        }
        const qry = '?light_id=';
        const queryIndex = url.lastIndexOf(qry);
        if (queryIndex !== -1) {
            this.selectedLightId = url.substring(queryIndex + qry.length);
            this.selectedLightUrl = url;
        }
        event.preventDefault();
    }

    onQrCodeClear(event: MouseEvent) {
        this.selectedLightId = undefined;
        this.selectedLightUrl = undefined;
    }

    /* Actions */

    /**
     * Publish a Call event to wwitch on/off lights with the selected parameters
     * matching the selected context filter.
     */
    switchLights() {
        this.controller!.switchLights(this.createContextFilter(), this.onOff, this.luminosity, this.effectiveColor, this.switchTime);
    }

    /**
     * View the JS code of the remote action for the selected filter and
     * parameters.
     */
    openCodeViewer() {
        this.bottomSheet.open(CodeViewerBottomSheetComponent, {
            data: this.getFormattedActionDetails(
                this.controller!.nextActionId,
                this.controller!.actionSource,
                this.onOff,
                this.luminosity,
                this.effectiveColor,
                this.switchTime,
                this.createContextFilter()),
        });
    }

    /**
     * Create a new light view that opens in a separate browser popup window.
     *
     * Newly created light popups are tiled on the screen starting at the upper
     * left corner.
     */
    openLightApp() {
        this.openLightAppInPopup();
    }

    showActionDetails(entry: ActionLogEntry) {
        this.bottomSheet.open(CodeViewerBottomSheetComponent, {
            data: this.getFormattedActionDetails(
                entry.action.getId(),
                entry.action.getSource(),
                entry.actionParams.on as boolean,
                entry.actionParams.luminosity as number,
                entry.actionParams.color as ColorRgba,
                entry.actionParams.switchTime as number,
                entry.actionParams.contextFilter as LightContextFilter),
        });
    }

    /**
     * Clear all entries in the action log view.
     */
    clearActionLog() {
        this.controller?.clearActionLog();
    }

    /* Public getters */

    get luminosity() {
        return this.luminosityPercent / 100;
    }

    get effectiveColor() {
        if (this.selectedCustomColor) {
            return this.selectedCustomColor.rgba;
        }
        const primary = this.colorPositionToRgba(this.primaryColorPosition)!;
        // Provide a semitransparent color so that the light bulb's interior
        // remains visible :-).
        primary[3] = 0.75;
        return primary;
    }

    colorRgbaToCssRgba(color: ColorRgba) {
        return `rgba(${color[0]}, ${color[1]}, ${color[2]}, ${color[3]})`;
    }

    getActionResultCount(log: Array<ActionLogEntry>) {
        return log.reduce((prev, e) => prev + e.actionResults.length, 0);
    }

    trackByActionLogEntries(index: number, entry: ActionLogEntry): string {
        return entry.action.getId();
    }

    /* Clock management */

    private startClock() {
        let currentTime = Date.now();
        this.currentClock$.next(currentTime);
        this.isClockStopped = false;
        const clockLoop = () => {
            if (this.isClockStopped) {
                return;
            }
            requestAnimationFrame(() => {
                const now = Date.now();
                if (now - currentTime >= 1000) {
                    this.currentClock$.next(currentTime = now);
                }
                clockLoop();
            });
        };
        clockLoop();
    }

    private stopClock() {
        this.isClockStopped = true;
    }

    /* Context filter */

    private initContextFilterBindings(ranges: LightContextRanges, options: any) {
        const range = (min: number, max: number): number[] => {
            return Array.from({ length: max - min + 1 }, (x, i) => i + min);
        };

        this.availableBuildings = range(ranges.building.min, ranges.building.max);
        this.availableFloors = range(ranges.floor.min, ranges.floor.max);
        this.availableRooms = range(ranges.room.min, ranges.room.max);

        this.selectedBuildings = options.initialContextFilterBuildings;
        this.selectedFloors = options.initialContextFilterFloors;
        this.selectedRooms = options.initialContextFilterRooms;
    }

    private createContextFilter(): LightContextFilter {
        if (this.selectedLightId) {
            return {
                lightId: this.selectedLightId,
            };
        }
        return {
            buildings: this.selectedBuildings,
            floors: this.selectedFloors,
            rooms: this.selectedRooms,
        };
    }

    /* Operation Parameters */

    private initOperationParams(options: any) {
        this.onOff = options.initialOpParamOnOff;
        this.luminosityPercent = options.initialOpParamLuminosity * 100;
        this.primaryColorPosition = this.rgbaToColorPosition(options.initialOpParamPrimaryColor);
        this.customColors = options.customColors;
        this.switchTime = options.initialSwitchTime;
    }

    /* Controller */

    private async initControlController() {
        // Connect the control assets provided by the controller to
        // corresponding data bindings of this view component.
        try {
            const opts = await this.config.getConfig();
            this.controller = new ControlController(opts);

            this.connectionInfo = this.controller.ddaEndpointUrl;
            this.initContextFilterBindings(opts.lightContextRanges, opts.control);
            this.initOperationParams(opts.control);
            this.actionLog$ = this.controller.actionLog$;
            this.changeRef.detectChanges();

            // Provide the app context with the container's identity ID to
            // be displayed in the title of the HTML document.
            this.appContext.setContext(`Light Control #${this.controller.id}`);
        } catch (error) {
            throw new Error(`ControlController couldn't be initialized: ${error}`);
        }
    }

    /* Open Light */

    private openLightAppInPopup() {
        const opts = this.controller?.options.control;
        const lw = opts.lightWindowWidth;
        const lh = opts.lightWindowHeight;
        const newWindow = window.open('./light', '_blank',
            `toolbar=no,resizable=no,status=no,location=no,menubar=no,titlebar=no,width=${lw},height=${lh}`);
        if (newWindow == null) {
            return;
        }
        const newWindowLayout = {
            screenLeft: newWindow.screenLeft,
            screenTop: newWindow.screenTop,
            outerWidth: newWindow.outerWidth,
            outerHeight: newWindow.outerHeight,
            availTop: (newWindow.screen as any).availTop || 0,
            availLeft: (newWindow.screen as any).availLeft || 0,
            availWidth: newWindow.screen.availWidth,
            availHeight: newWindow.screen.availHeight,
        };
        let nx: number;
        let ny: number;
        if (this.currentLightWindowLayout) {
            const cx = this.currentLightWindowLayout.screenLeft;
            const cy = this.currentLightWindowLayout.screenTop;
            const cw = this.currentLightWindowLayout.outerWidth;
            const ch = this.currentLightWindowLayout.outerHeight;
            const st = this.currentLightWindowLayout.availTop;
            const sl = this.currentLightWindowLayout.availLeft;
            const sw = this.currentLightWindowLayout.availWidth;
            const sh = this.currentLightWindowLayout.availHeight;
            nx = cx;
            ny = cy + ch;
            if (ny + ch >= st + sh) {
                nx += cw;
                ny = st;
                if (nx + cw >= sl + sw) {
                    nx = sl;
                    ny = st;
                }
            }
        } else {
            nx = newWindowLayout.availLeft;
            ny = newWindowLayout.availTop;
        }
        newWindow.moveTo(nx, ny);
        newWindowLayout.screenLeft = newWindow.screenLeft;
        newWindowLayout.screenTop = newWindow.screenTop;
        this.currentLightWindowLayout = newWindowLayout;
    }

    /* Color manipulation */

    private updateColorSliderThumb(position: number) {
        // If position is invalid, use transparent white to make thumb
        // invisible.
        const color = this.colorPositionToRgba(position) ?? [255, 255, 255, 0] as ColorRgba;
        const cardElem = this.cardElementRef.nativeElement as HTMLElement;
        const labelColor = this.colorRgbaToCssRgba(color);
        const sliderThumbLabel = cardElem.querySelector('.control-color-slider .mdc-slider__thumb .mdc-slider__value-indicator-container .mdc-slider__value-indicator') as HTMLElement;
        if (sliderThumbLabel) {
            sliderThumbLabel.style.backgroundColor = labelColor;
            // var --mdc-slider-label-container-color is defined by ::before element.
            sliderThumbLabel.style.setProperty('--mdc-slider-label-container-color', labelColor);
        }
    }

    /**
     * Converts an rgba tuple to the color position of the slider, if possible.
     * For colors which are not part of the slider's color gradient, -1 is
     * returned.
     *
     * The following color stops are used, as defined in the linear color
     * gradient of the .control-color-slider style in control.component.scss.
     *
     * [255, 0, 0],
     * [255, 255, 0],
     * [0, 255, 0],
     * [0, 255, 255],
     * [0, 0, 255],
     * [255, 0, 255],
     * [255, 0, 0]
     */
    private rgbaToColorPosition(rgba: ColorRgba): number {
        const [r, g, b, a] = rgba;
        const d = 1 / 6;
        let v: number;

        if (r === 255 && b === 0) {
            v = d * g / 255;
        } else if (g === 255 && b === 0) {
            v = d + ((255 - d * r) / 255);
        } else if (r === 0 && g === 255) {
            v = 2 * d + (d * b / 255);
        } else if (r === 0 && b === 255) {
            v = 3 * d + ((255 - d * g) / 255);
        } else if (g === 0 && b === 255) {
            v = 4 * d + (d * r / 255);
        } else if (r === 255 && g === 0) {
            v = 5 * d + ((255 - d * b) / 255);
        } else {
            return -1;
        }

        return v * this.primaryColorPositionMax;
    }

    /**
     * Converts a color slider position to the corresponding color RGBA tuple.
     * If -1 is passed in (a color not part of the slider's color gradient),
     * undefined is returned (see method rgbaToColorPosition).
     */
    private colorPositionToRgba(position: number): ColorRgba | undefined {
        if (position === -1) {
            return undefined;
        }

        let r = 0;
        let g = 0;
        let b = 0;
        const pos = position / this.primaryColorPositionMax;
        const d = 1 / 6;

        if (pos <= d) {
            r = 255;
            g = 255 * pos / d;
            b = 0;
        } else if (pos <= 2 * d) {
            r = 255 - (255 * (pos - d) / d);
            g = 255;
            b = 0;
        } else if (pos <= 3 * d) {
            r = 0;
            g = 255;
            b = (255 * (pos - 2 * d) / d);
        } else if (pos <= 4 * d) {
            r = 0;
            g = 255 - (255 * (pos - 3 * d) / d);
            b = 255;
        } else if (pos <= 5 * d) {
            r = (255 * (pos - 4 * d) / d);
            g = 0;
            b = 255;
        } else {
            r = 255;
            g = 0;
            b = 255 - (255 * (pos - 5 * d) / d);
        }

        return [Math.round(r), Math.round(g), Math.round(b), 1] as ColorRgba;
    }

    /* Code formatting */

    /**
     * Gets pretty printed code for JavaScript objects representing the
     * currently selected operation parameters including selected context
     * filter.
     *
     * Returns an object containing the properties `operation`, and
     * `operationParameters` with formatted multiline content strings.
     */
    private getFormattedActionDetails(id: Uuid, source: string, on: boolean, luminosity: number, color: ColorRgba, switchTime: number, contextFilter: LightContextFilter) {
        const arrayFormat = (v: Array<any>) => {
            return JSON.stringify(v).replace(/,/g, ', ');
        };
        const replacer = (_: any, v: any) => {
            // Format arrays that contain no other arrays as elements in a
            // single line.
            if (Array.isArray(v) && v.every((e => !Array.isArray(e)))) {
                return arrayFormat(v);
            }
            return v;
        };
        const fixer = (v: string) => {
            return v.replace(/\\/g, '')
                .replace(/\"\[/g, '[')
                .replace(/\]\"/g, ']')
                .replace(/\"\{/g, '{')
                .replace(/\}\"/g, '}');
        };
        const formatter = (v: any) => {
            return fixer(JSON.stringify(v, replacer, 2));
        };
        return {
            operation: formatter(this.controller!.options.lightControlOperation),
            operationId: formatter(id),
            operationSource: formatter(source),
            operationParameters: formatter({
                on,
                luminosity,
                color,
                switchTime,
                contextFilter,
            }),
        };
    }
}
