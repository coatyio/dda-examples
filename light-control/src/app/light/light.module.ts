/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { NgModule } from '@angular/core';

import { SharedModule } from '../shared/shared.module';
import { LightComponent } from './light.component';
import { LightRoutingModule } from './light-routing.module';

/**
 * Defines Angular specific configuration metadata of the lazy-loaded light
 * feature module.
 */
@NgModule({
    declarations: [
        LightComponent,
    ],
    imports: [
        SharedModule,
        LightRoutingModule,
    ],
})
export class LightModule { }
