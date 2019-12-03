# Feature Summary

## Features

### Send notifications from Jira to Mattermost

Notify your team of the latest updates by sending notifications from your Jira projects to Mattermost channels.

![image](https://user-images.githubusercontent.com/13119842/59113100-6cd7a800-8912-11e9-9e23-3639c0eb9c4d.png)

![image](https://user-images.githubusercontent.com/13119842/59113138-7f51e180-8912-11e9-9fc5-3077ba90a8a8.png)

Notifications are configured with webhooks and offer full JQL support. Configuration is restricted to Jira System Admins only. See [these instructions]() for install and configuration.

The following Jira event notifications are supported:

* Issue created
* Issue updated, including when an issue is reopened or resolved, or when the assignee is changed
* Issue deleted when not yet resolved
* Comments created, updated or deleted

If you’d like to see support for additional events, [let us know](https://mattermost.uservoice.com/forums/306457-general).

### 1.2 Create and manage Jira issues in Mattermost

Connect your Mattermost account to Jira via `/jira connect` command, then create and manage issues across Mattermost channels. You can disconnect your account anytime via `/jira disconnect`.

#### 1.2.1 Create Jira issues

Create Jira issues from a Mattermost message by clicking the **More Actions** \(...\) option of any message in the channel \(available when you hover over a message\), then selecting **Create Jira Issue**.

Then, on the resulting issue creation dialog, select the project, issue type and enter other fields to create the issue.

![image](https://user-images.githubusercontent.com/13119842/59113188-985a9280-8912-11e9-9def-9a7382b4137e.png)

Click **Create** and the Jira issue is now created, including any file attachments part of the Mattermost message.

![image](https://user-images.githubusercontent.com/13119842/59113219-a4deeb00-8912-11e9-9741-5ddc8a4b51fa.png)

**NOTE**: This plugin does not support all Jira fields. If the project you tried to create an issue for has **required fields** not yet supported, you will be prompted to manually create an issue. Clicking the provided link brings you to an issue creation screen in Jira, with the fields entered previously pre-filled.

The supported Jira fields are:

* **Project Picker**: Custom fields and the built-in **Project** field.
* **Single-Line Text**: Custom fields, and built-in fields such as **Summary**, **Environment**.
* **Multi-Line Text**: Custom fields, and built-in fields such as **Description**.
* **Single-Choice Issue**: Custom fields, and built-in fields such as **Issue Type** and **Priority**. 

#### 1.2.2 Attach Messages to Jira Issues

Keep all information in one place by attaching parts of Mattermost conversations in Jira issues as comments. To attach a message, click the **More Actions** \(...\) option of any message in the channel \(available when you hover over a message\), then select **Attach to Jira Issue**.

Then, on the resulting dialog, select the issue you want to attach it to. You may search for issues containing specific text.

![image](https://user-images.githubusercontent.com/13119842/59113267-b627f780-8912-11e9-90ec-417d430de7e6.png)

Click **Attach** and the message is attached to the selected Jira issue as a comment.

#### 1.2.3 Transition Jira issues

Transition issues without the need to switch to your Jira project. To transition an issue, use the `/jira transition <issue-key> <state>` command.

For instance, `/jira transition EXT-20 done` transitions the issue key **EXT-20** to **Done**.

![image](https://user-images.githubusercontent.com/13119842/59113377-dfe11e80-8912-11e9-8971-f869fa123366.png)

Note

* States and issue transitions are based on your Jira project workflow configuration. If an invalid state is entered, an ephemeral message is returned mentioning that the state couldn't be found.
* Partial Matches work.  For example, typing `/jira transition EXT-20 in`  will transition to "In Progress".  However, if there are states of "In Review, In Progress", the plugin bot will ask you to be more specific and display the partial matches.

#### 1.2.4 Assign Jira issues

Assign issues to other Jira users without the need to switch to your Jira project. To assign an issue, use the /jira assign  .

For instance, `/jira assign EXT-20 john` transitions the issue key **EXT-20** to **John**.

Note

* Partial Matches work with Usernames and Firstname/Lastname

