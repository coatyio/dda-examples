/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { Component, Host, AfterViewInit, OnDestroy } from '@angular/core';

import { MatPseudoCheckboxState } from '@angular/material/core';
import { MatSelect } from '@angular/material/select';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';

/**
 * A replacement for the yet missing "select/deselect all" option for mat-select
 * with multiple selection (see
 * https://stackblitz.com/edit/angular-material-abra-select-all?file=app%2Fmat-option-select-all%2Fmat-option-select-all.component.ts).
 *
 * Should be replaced by the official Angular Material version when available.
 */
@Component({
    selector: 'app-mat-option-select-all',
    template: `
    <div class="mat-mdc-option" (click)="onSelectAllClick($event)">
      <mat-pseudo-checkbox [state]="state"></mat-pseudo-checkbox>
      <span class="mat-mdc-option-text">Select all</span>
    </div>
  `,
    styles: [`
    .mat-mdc-option {
      border-bottom: 1px solid rgba(0, 0, 0, 0.12);
      height: 3.5em;
      line-height: 3.5em;
  }`]
})
export class MatOptionSelectAllComponent implements AfterViewInit, OnDestroy {

    state: MatPseudoCheckboxState = 'checked';

    private options: any[] = [];
    private value: any[] = [];

    private destroyed = new Subject<void>();

    constructor(@Host() private matSelect: MatSelect) { }

    ngAfterViewInit() {
        this.options = this.matSelect.options.map(x => x.value);
        this.matSelect.options.changes
            .pipe(takeUntil(this.destroyed))
            .subscribe(res => {
                this.options = this.matSelect.options.map(x => x.value);
                this.updateState();
            });

        this.value = this.matSelect.ngControl.control!.value;
        this.matSelect.ngControl.valueChanges!
            .pipe(takeUntil(this.destroyed))
            .subscribe(res => {
                this.value = res;
                this.updateState();
            });
        // Avoid expression changed after check error
        setTimeout(() => {
            this.updateState();
        });
    }

    ngOnDestroy() {
        this.destroyed.next();
        this.destroyed.complete();
    }

    onSelectAllClick(evt: MouseEvent) {
        if (this.state === 'checked') {
            this.matSelect.ngControl.control!.setValue([]);
        } else {
            this.matSelect.ngControl.control!.setValue(this.options);
        }
    }

    private updateState() {
        const areAllSelected = this.areArraysEqual(this.value, this.options);
        if (areAllSelected) {
            this.state = 'checked';
        } else if (this.value.length > 0) {
            this.state = 'indeterminate';
        } else {
            this.state = 'unchecked';
        }
    }

    private areArraysEqual(a: any[], b: any[]) {
        return [...a].sort().join(',') === [...b].sort().join(',');
    }

}
