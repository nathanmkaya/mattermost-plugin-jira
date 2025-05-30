// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';

import {closeConnectModal, redirectConnect} from 'actions';
import {getInstalledInstances, getUserConnectedInstances, isConnectModalVisible} from 'selectors';

import {GlobalState} from 'types/store';

import ConnectModal from './connect_modal';

const mapStateToProps = (state: GlobalState) => {
    return {
        visible: isConnectModalVisible(state),
        connectedInstances: getUserConnectedInstances(state),
        installedInstances: getInstalledInstances(state),
    };
};

const mapDispatchToProps = (dispatch) => bindActionCreators({
    closeModal: closeConnectModal,
    redirectConnect,
}, dispatch);

export default connect(mapStateToProps, mapDispatchToProps)(ConnectModal);
