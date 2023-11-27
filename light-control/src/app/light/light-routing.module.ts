/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';

import { LightComponent } from './light.component';

const routes: Routes = [
    { path: '', component: LightComponent }
];

/**
 * Module definition for configuring the routes of the light module.
 */
@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class LightRoutingModule { }
