/*!
 * SPDX-FileCopyrightText: © 2023 Siemens AG
 * SPDX-License-Identifier: MIT
 */

import { Pipe, PipeTransform } from '@angular/core';

/**
 * Converts any input value except `undefined` and `null` into the date, the
 * input has been received.
 */
@Pipe({
    name: 'dateNow'
})
export class DateNowPipe implements PipeTransform {

    transform(value: any, args?: any): Date | undefined {
        if (value === null || value === undefined) {
            return undefined;
        }
        return new Date();
    }

}
