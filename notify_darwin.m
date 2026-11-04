// Native macOS notifications. They show the honk.app icon, and a click opens
// the PR. They only work when honk runs from inside honk.app.

#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

#include "_cgo_export.h"

static void honkLog(NSString *msg) { honkNotifyLog((char *)msg.UTF8String); }

@interface HonkNotifyDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation HonkNotifyDelegate
// Show the banner even when honk is the active app.
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))done {
  done(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList);
}

// A click on the notification opens the PR.
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
    didReceiveNotificationResponse:(UNNotificationResponse *)response
             withCompletionHandler:(void (^)(void))done {
  NSString *u = response.notification.request.content.userInfo[@"url"];
  if ([u isKindOfClass:[NSString class]] && [u hasPrefix:@"https://github.com/"]) {
    NSURL *url = [NSURL URLWithString:u];
    if (url != nil) {
      [[NSWorkspace sharedWorkspace] openURL:url];
    }
  }
  done();
}
@end

static HonkNotifyDelegate *delegate;

// honkNotifyInit returns 1 when native notifications can work.
int honkNotifyInit(void) {
  if ([[NSBundle mainBundle] bundleIdentifier] == nil) {
    return 0;  // not inside an app bundle
  }
  UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
  delegate = [HonkNotifyDelegate new];
  center.delegate = delegate;
  [center requestAuthorizationWithOptions:UNAuthorizationOptionAlert
                        completionHandler:^(BOOL granted, NSError *err) {
                          if (err != nil) {
                            honkLog([NSString stringWithFormat:@"authorization error: %@", err.localizedDescription]);
                          } else {
                            honkLog(granted ? @"notifications allowed" : @"notifications not allowed");
                          }
                        }];
  return 1;
}

void honkNotify(const char *title, const char *body, const char *url) {
  UNMutableNotificationContent *c = [UNMutableNotificationContent new];
  c.title = [NSString stringWithUTF8String:title];
  c.body = [NSString stringWithUTF8String:body];
  c.userInfo = @{@"url" : [NSString stringWithUTF8String:url]};
  // No sound: honk plays its own.
  UNNotificationRequest *req = [UNNotificationRequest requestWithIdentifier:[NSUUID UUID].UUIDString
                                                                    content:c
                                                                    trigger:nil];
  [[UNUserNotificationCenter currentNotificationCenter]
      addNotificationRequest:req
       withCompletionHandler:^(NSError *err) {
         if (err != nil) {
           honkLog([NSString stringWithFormat:@"post error: %@", err.localizedDescription]);
         }
       }];
}
