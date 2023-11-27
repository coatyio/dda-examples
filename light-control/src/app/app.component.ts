/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { Component, OnDestroy } from '@angular/core';
import { Title } from '@angular/platform-browser';
import { Subscription } from 'rxjs';

import { AppContextService } from './app-context.service';

@Component({
    selector: 'app-root',
    templateUrl: './app.component.html',
})
export class AppComponent implements OnDestroy {

    private contextSubscription: Subscription;

    constructor(titleService: Title, appContext: AppContextService) {
        // Set up a context subscription to update the title of the web apps's
        // HTML document depending on the currently loaded module.
        this.contextSubscription = appContext.context$.subscribe(context => titleService.setTitle(context));

        // Provide an initial context for the app. This context will be
        // overridden when the light or control module is being loaded lazily.
        appContext.setContext("Light Operations");
    }

    ngOnDestroy() {
        // Ensure to dispose resources held by the context subscription after
        // this component is destroyed.
        this.contextSubscription?.unsubscribe();
    }
}
