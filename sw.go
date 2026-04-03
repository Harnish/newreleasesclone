package main

import (
	"fmt"
	"net/http"
)

var serviceWorkerJS = `
self.addEventListener('push', function(e) {
    var d = {};
    try { d = e.data.json(); } catch(_) {}
    e.waitUntil(
        self.registration.showNotification(d.title || 'New Release', {
            body: d.body || '',
            data: { url: d.url || '/' }
        })
    );
});

self.addEventListener('notificationclick', function(e) {
    e.notification.close();
    var url = (e.notification.data && e.notification.data.url) || '/';
    e.waitUntil(clients.openWindow(url));
});
`

func handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Service-Worker-Allowed", "/")
	fmt.Fprint(w, serviceWorkerJS)
}
