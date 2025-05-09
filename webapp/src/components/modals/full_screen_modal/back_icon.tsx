// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';

type Props = {
    className?: string;
}

export default class BackIcon extends React.PureComponent<Props> {
    render(): JSX.Element {
        return (
            <button
                {...this.props}
            >
                <FormattedMessage
                    id='generic_icons.back'
                    defaultMessage='Back Icon'
                >
                    {(ariaLabel: string): JSX.Element => (
                        <svg
                            width='24px'
                            height='24px'
                            viewBox='0 0 24 24'
                            role='icon'
                            aria-label={ariaLabel}
                        >
                            <path d='M20,11V13H8L13.5,18.5L12.08,19.92L4.16,12L12.08,4.08L13.5,5.5L8,11H20Z'/>
                        </svg>
                    )}
                </FormattedMessage>
            </button>
        );
    }
}
