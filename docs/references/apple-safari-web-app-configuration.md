# Configuring Web Applications (Apple Safari)

> Source: https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html
> Copyright 2016 Apple Inc. All Rights Reserved.
> Last Updated: 2016-12-12

A web application is designed to look and behave in a way similar to a native application—for example, it is scaled to fit the entire screen on iOS. You can tailor your web application for Safari on iOS even further, by making it appear like a native application when the user adds it to the Home screen. You do this by using settings for iOS that are ignored by other platforms.

## Specifying a Webpage Icon for Web Clip

You may want users to be able to add your web application or webpage link to the Home screen. These links, represented by an icon, are called Web Clips.

### Website-wide icon
Place an icon file in PNG format in the root document folder called `apple-touch-icon.png`

### Page-specific icon
Add a link element to the webpage:
```html
<link rel="apple-touch-icon" href="/custom_icon.png">
```

### Multiple icons for different device resolutions
Add a `sizes` attribute to each link element:
```html
<link rel="apple-touch-icon" href="touch-icon-iphone.png">
<link rel="apple-touch-icon" sizes="152x152" href="touch-icon-ipad.png">
<link rel="apple-touch-icon" sizes="180x180" href="touch-icon-iphone-retina.png">
<link rel="apple-touch-icon" sizes="167x167" href="touch-icon-ipad-retina.png">
```

The icon that is the most appropriate size for the device is used. If there is no icon that matches the recommended size:
1. The smallest icon larger than the recommended size is used
2. If there are no icons larger than the recommended size, the largest icon is used

If no icons are specified using a link element, the website root directory is searched for icons with the `apple-touch-icon...` prefix.

## Specifying a Launch Screen Image

On iOS, you can specify a launch screen image that is displayed while your web application launches. This is especially useful when your web application is offline. By default, a screenshot of the web application the last time it was launched is used.

```html
<link rel="apple-touch-startup-image" href="/launch.png">
```

## Adding a Launch Icon Title

On iOS, you can specify a web application title for the launch icon. By default, the `<title>` tag is used.

```html
<meta name="apple-mobile-web-app-title" content="AppTitle">
```

## Hiding Safari User Interface Components (Standalone Mode)

When you use standalone mode, Safari is not used to display the web content—specifically, there is no browser URL text field at the top of the screen or button bar at the bottom of the screen. Only a status bar appears at the top of the screen.

```html
<meta name="apple-mobile-web-app-capable" content="yes">
```

You can determine whether a webpage is displaying in standalone mode using the `window.navigator.standalone` read-only Boolean JavaScript property.

## Changing the Status Bar Appearance

If your web application displays in standalone mode, you can minimize the status bar that is displayed at the top of the screen on iOS.

**Note:** This meta tag has no effect unless you first specify standalone mode.

```html
<meta name="apple-mobile-web-app-status-bar-style" content="black">
```

Available values:
- `default` - Default status bar appearance
- `black` - Black background
- `black-translucent` - Translucent black (content can appear behind status bar)

## Linking to Other Native Apps

Your web application can link to other built-in iOS apps by creating a link with a special URL:

### Phone calls
```html
<a href="tel:1-408-555-5555">Call me</a>
```

### SMS/iMessage
```html
<a href="sms:1-408-555-5555">Text me</a>
```

## Complete Example

```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">

    <!-- Web App Configuration -->
    <meta name="apple-mobile-web-app-capable" content="yes">
    <meta name="apple-mobile-web-app-status-bar-style" content="black">
    <meta name="apple-mobile-web-app-title" content="My App">

    <!-- Icons -->
    <link rel="apple-touch-icon" href="touch-icon-iphone.png">
    <link rel="apple-touch-icon" sizes="152x152" href="touch-icon-ipad.png">
    <link rel="apple-touch-icon" sizes="180x180" href="touch-icon-iphone-retina.png">
    <link rel="apple-touch-icon" sizes="167x167" href="touch-icon-ipad-retina.png">

    <!-- Launch Screen -->
    <link rel="apple-touch-startup-image" href="/launch.png">

    <title>My Web App</title>
</head>
<body>
    <!-- Content -->
</body>
</html>
```
