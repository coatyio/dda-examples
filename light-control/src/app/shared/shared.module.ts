/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { NgModule, ModuleWithProviders } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { HttpClientModule } from '@angular/common/http';

import { QRCodeModule } from 'angularx-qrcode';

// Angular Material Modules
import { MatBottomSheetModule } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDividerModule } from '@angular/material/divider';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatIconModule } from '@angular/material/icon';
import { MatListModule } from '@angular/material/list';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatSelectModule } from '@angular/material/select';
import { MatSliderModule } from '@angular/material/slider';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';

// Used by MatOptionSelectAllComponent
import { MatPseudoCheckboxModule } from '@angular/material/core';
import { MatOptionSelectAllComponent } from './mat-option-select-all.component';

import { DateNowPipe } from './date-now.pipe';

/**
 * A shared module that is imported by the lazy-loaded feature modules `light`
 * and `control`.
 */
@NgModule({
    declarations: [
        DateNowPipe,
        MatOptionSelectAllComponent,
    ],
    imports: [
        CommonModule,
        FormsModule,
        HttpClientModule,
        QRCodeModule,

        // Used by MatOptionSelectAllComponent
        MatPseudoCheckboxModule,

        MatBottomSheetModule,
        MatButtonModule,
        MatButtonToggleModule,
        MatCardModule,
        MatCheckboxModule,
        MatDividerModule,
        MatExpansionModule,
        MatIconModule,
        MatListModule,
        MatToolbarModule,
        MatTooltipModule,
        MatSelectModule,
        MatSliderModule,
        MatSlideToggleModule,
    ],
    exports: [
        CommonModule,
        FormsModule,
        HttpClientModule,
        QRCodeModule,
        DateNowPipe,

        // Used by MatOptionSelectAllComponent
        MatPseudoCheckboxModule,
        MatOptionSelectAllComponent,

        MatBottomSheetModule,
        MatButtonModule,
        MatButtonToggleModule,
        MatCardModule,
        MatCheckboxModule,
        MatDividerModule,
        MatExpansionModule,
        MatIconModule,
        MatListModule,
        MatToolbarModule,
        MatTooltipModule,
        MatSelectModule,
        MatSliderModule,
        MatSlideToggleModule,
    ]
})
export class SharedModule {
    static forRoot(): ModuleWithProviders<SharedModule> {
        return {
            ngModule: SharedModule,
            providers: []
        };
    }
}
