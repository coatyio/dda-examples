/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { ChangeDetectionStrategy, Component, OnDestroy, ViewEncapsulation } from '@angular/core';
import { Location } from '@angular/common';
import { Observable } from 'rxjs';
import { skip } from 'rxjs/operators';

import { AppContextService } from '../app-context.service';
import { Light, LightContext, LightContextRanges } from '../shared/light.model';
import { LightControlConfigService } from '../shared/light-control-config.service';
import { LightController } from './light.controller';

/**
 * An Angular component that provides a view of a single light with its current
 * light status and light context. The light context properties can be changed
 * interactively by the user.
 */
@Component({
    selector: 'app-light',
    templateUrl: 'light.component.html',
    styleUrls: ['light.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,

    // Required for angularx-qrcode component styling (see css class .light-map-card-image-qrcode-div).
    // The inner div is not accessible with default view encapsulation as this div's CSS is not scoped
    // with an Angular generated CSS attribute.
    encapsulation: ViewEncapsulation.None,
})
export class LightComponent {

    /** The Light object. */
    light?: Light;

    /** Light color changes emitted by an observable. */
    lightColor$?: Observable<string>;

    /** Real light switch changes emitted by an observable, but the initial color change is not emitted. */
    lastSwitched$?: Observable<string>;

    /** The current light context. */
    lightContext?: LightContext;

    /** Ranges for light context parameters. */
    lightContextRanges?: LightContextRanges;

    /** Absolute URL pointing to light control UI with lightId query param. */
    qrCodeUrl?: string;

    /** DDA Endpoint address. */
    connectionInfo?: string

    constructor(public appContext: AppContextService, private location: Location, private config: LightControlConfigService) {
        this.appContext.setContext('Light');
        this.initNgModelBindings();
        this.initLightController();
    }

    onQrCodeClick(event: MouseEvent) {
        // Open a new light control UI with the context filter preset to this light.
        window.open(this.qrCodeUrl, '_blank');
    }

    onQrCodeDrag(event: DragEvent, tooltip: any) {
        tooltip.hide();
        event.dataTransfer!.setData('text/plain', this.qrCodeUrl!);
        event.dataTransfer!.setData('text/qrcode', this.qrCodeUrl!);
    }

    getQrCodeWidth() {
        // QrCodeComponent also accepts string percentages in addition to pixel
        // values.
        return '100%' as unknown as number;
    }

    /**
     * Get absolute URL pointing to light control UI with lightId query param set.
     */
    private getQrCodeUrl() {
        const urlPath = this.location.prepareExternalUrl(`/control?light_id=${this.light ? this.light.id : ''}`);
        return window.location.protocol + '//' + window.location.host + urlPath;
    }

    private initNgModelBindings() {
        // Initialize a default context as long as real context is not yet emitted
        // so that ngModel bindings do not throw error initially.
        this.lightContext = { building: 0, floor: 0, room: 0 };
        this.lightContextRanges = {
            building: { min: 0, max: 0, tickInterval: 1 },
            floor: { min: 0, max: 0, tickInterval: 1 },
            room: { min: 0, max: 0, tickInterval: 1 }
        };
    }

    private async initLightController() {
        // Connect the light assets provided by the controller to corresponding
        // data bindings of this view component.
        try {
            const opts = await this.config.getConfig();
            const ctrl = new LightController(opts);

            this.lightContextRanges = opts.lightContextRanges;
            this.light = ctrl.light;
            this.lightContext = ctrl.lightContext;
            this.lightColor$ = ctrl.lightColorChange$;
            this.lastSwitched$ = ctrl.lightColorChange$.pipe(skip(1));
            this.qrCodeUrl = this.getQrCodeUrl();
            this.connectionInfo = ctrl.ddaEndpointUrl;

            // Provide the app context with the light ID to be displayed in
            // the title of the HTML document.
            this.appContext.setContext(`Light #${this.light.id}`);
        } catch (error) {
            throw new Error(`LightController couldn't be initialized: ${error}`);
        }
    }

}
