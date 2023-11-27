/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { Injectable } from '@angular/core';
import { Location } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { lastValueFrom } from 'rxjs';
import { catchError } from 'rxjs/operators';

import { SharedModule } from "./shared.module";

/**
 * An app-wide service that provides light control configuration for a specific
 * feature module, i.e. light or control module.
 */
@Injectable({
    providedIn: SharedModule,
})
export class LightControlConfigService {

    private readonly configUrl = '/assets/config/light-control.config.json';

    constructor(private http: HttpClient, private location: Location) { }

    /**
     * Gets the light control configuration.
     *
     * @returns a promise resolving to a plain JS configuration object
     * according to assets/config/light-control.config.json
     */
    async getConfig(): Promise<any> {
        return lastValueFrom(
            // Auto-prefix the resource URL path by base href to yield a host
            // specific external URL.
            this.http.get(this.location.prepareExternalUrl(this.configUrl))
                .pipe(
                    catchError(error => {
                        console.error(`Could not retrieve ${this.configUrl} from host: `, error);
                        throw new Error(error.toString());
                    }),
                ));
    }
}
